package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
)

// RateSource defines the interface for rate providers
type RateSource interface {
	FetchRates() ([]Rate, error)
}

// Global storage for latest rates
var (
	latestRates       []Rate
	ratesMutex        sync.RWMutex
	lastFetchTime     time.Time
	cacheDuration     = 5 * time.Minute
	lendingThresholds = map[string]float64{
		"USDC":  30.0,
		"TIA":   30.0,
		"USDT":  30.0,
		"FDUSD": 30.0,
	}
	db                  *Database
	userPreferences     = make(map[int64]bool)             // Store user preferences for CEX rates
	previousRates       = make(map[string]map[string]Rate) // token -> source -> rate
	rateChangeThreshold = 5.0                              // 5% change threshold
)

// updateLatestRates updates the global rates storage thread-safely
func updateLatestRates(rates []Rate) {
	ratesMutex.Lock()
	defer ratesMutex.Unlock()
	latestRates = rates
	lastFetchTime = time.Now()
}

// getLatestRates retrieves the latest rates thread-safely
func getLatestRates() []Rate {
	ratesMutex.RLock()
	defer ratesMutex.RUnlock()
	return latestRates
}

// isCacheValid checks if the cache is valid
func isCacheValid() bool {
	ratesMutex.RLock()
	defer ratesMutex.RUnlock()
	return !lastFetchTime.IsZero() && time.Since(lastFetchTime) < cacheDuration
}

// fetchRates fetches rates from multiple sources
func fetchRates(sources ...RateSource) ([]Rate, error) {
	log.Printf("Fetching rates from %d sources...", len(sources))
	var allRates []Rate
	var errors []error

	for _, source := range sources {
		rates, err := source.FetchRates()
		if err != nil {
			log.Printf("Error fetching rates from %T: %v", source, err)
			errors = append(errors, err)
			continue
		}

		// Convert APR to APY for Neptune and Injera rates
		for i := range rates {
			switch rates[i].Source {
			case "Neptune", "Injera":
				// Assuming daily compounding (365 times per year)
				rates[i].LendingRate = convertAPRtoAPY(rates[i].LendingRate, 365)
				rates[i].BorrowRate = convertAPRtoAPY(rates[i].BorrowRate, 365)
			}
		}

		// Log rates from this source
		for _, rate := range rates {
			log.Printf("Fetched rate from %s: %s L: %.2f%% B: %.2f%%",
				rate.Source, rate.Token, rate.LendingRate, rate.BorrowRate)
		}

		allRates = append(allRates, rates...)
	}

	if len(allRates) == 0 && len(errors) > 0 {
		return nil, fmt.Errorf("all sources failed to fetch rates")
	}

	log.Printf("Successfully fetched %d rates total", len(allRates))
	updateLatestRates(allRates)
	return allRates, nil
}

// getRatesWithCache fetches rates with caching
func getRatesWithCache(sources ...RateSource) ([]Rate, error) {
	if isCacheValid() {
		return getLatestRates(), nil
	}
	return fetchRates(sources...)
}

var commandHelp = map[string]string{
	"/start": "Subscribe to rate notifications",
	"/stop":  "Unsubscribe from rate notifications",
	"/rate":  "Show current rates for all tokens\nUsage: /rate [token]\nExample: /rate USDT",
	"/help":  "Show this help message",
	"/cex":   "Toggle visibility of CEX (Centralized Exchange) rates",
}

func getHelpMessage() string {
	var message strings.Builder
	message.WriteString("*Available Commands*\n\n")

	// Sort commands for consistent ordering
	var commands []string
	for cmd := range commandHelp {
		commands = append(commands, cmd)
	}
	sort.Strings(commands)

	for _, cmd := range commands {
		message.WriteString(fmt.Sprintf("*%s*\n%s\n\n", cmd, commandHelp[cmd]))
	}

	return message.String()
}

func shouldShowCEXRates(chatID int64) bool {
	preference, exists := userPreferences[chatID]
	if !exists {
		// Try to load from database
		dbPreference, err := db.GetShowCEX(chatID)
		if err != nil {
			log.Printf("Error loading CEX preference: %v", err)
			return true // Default to showing CEX rates
		}
		userPreferences[chatID] = dbPreference
		return dbPreference
	}
	return preference
}

func hasSignificantChange(oldRate, newRate Rate) bool {
	if oldRate.LendingRate == 0 {
		return true // First time seeing this rate
	}

	percentChange := ((newRate.LendingRate - oldRate.LendingRate) / oldRate.LendingRate) * 100
	return math.Abs(percentChange) >= rateChangeThreshold
}

func updatePreviousRates(rates []Rate) {
	ratesMutex.Lock()
	defer ratesMutex.Unlock()

	for _, rate := range rates {
		if _, exists := previousRates[rate.Token]; !exists {
			previousRates[rate.Token] = make(map[string]Rate)
		}
		previousRates[rate.Token][rate.Source] = rate
	}
}

func main() {
	// Try to load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, will use OS environment variables")
	}

	// Get Telegram token
	telegramToken := getEnv("TELEGRAM_TOKEN", "")
	if telegramToken == "" {
		log.Fatal("TELEGRAM_TOKEN is not set")
	}

	bot, err := tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		log.Panic(err)
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	// Create an update config with a 60-second timeout
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	// Start receiving updates
	updates := bot.GetUpdatesChan(updateConfig)

	// Initialize database
	dbPath := filepath.Join("data", "bot.db")
	// Ensure data directory exists
	err = os.MkdirAll("data", 0755)
	if err != nil {
		log.Fatal("Failed to create data directory:", err)
	}

	db, err = NewDatabase(dbPath)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()

	// Load existing subscribers
	activeChatIDs, err := db.GetAllSubscribers()
	if err != nil {
		log.Fatal("Failed to load subscribers:", err)
	}

	// Initialize sources
	okxSource := NewOKXSource()
	neptuneSource := NewNeptuneSource()
	injeraSource := NewInjeraSource()
	binanceSource := NewBinanceSimpleEarnSource()
	bybitSource := NewBybitSource()

	// Sources are already initialized with their categories in their respective New functions
	sources := []RateSource{okxSource, neptuneSource, injeraSource, binanceSource, bybitSource}

	// Function to fetch and process rates
	cronFetchRates := func() {
		rates, err := fetchRates(sources...)
		if err != nil {
			log.Printf("Error fetching rates: %v", err)
			return
		}

		// Filter rates based on lending rate thresholds and significant changes
		filteredRates := []Rate{}
		for _, rate := range rates {
			threshold, exists := lendingThresholds[rate.Token]
			if !exists {
				continue
			}

			// Check if rate exceeds threshold and has significant change
			if rate.LendingRate >= threshold {
				prevRate, hasPrevious := previousRates[rate.Token][rate.Source]
				if !hasPrevious || hasSignificantChange(prevRate, rate) {
					filteredRates = append(filteredRates, rate)
				}
			}
		}

		// Update previous rates after filtering
		updatePreviousRates(rates)

		if len(filteredRates) > 0 {
			// Group rates by token
			ratesByToken := make(map[string][]Rate)
			tokensWithHighRates := make(map[string]bool)

			// First, identify tokens with high rates
			for _, rate := range filteredRates {
				tokensWithHighRates[rate.Token] = true
			}

			// Then, collect all rates for those tokens
			for _, rate := range rates {
				if tokensWithHighRates[rate.Token] {
					ratesByToken[rate.Token] = append(ratesByToken[rate.Token], rate)
				}
			}

			// Send notification to all active chat IDs
			for chatID := range activeChatIDs {
				var message strings.Builder

				// Build message based on user preferences
				for token := range tokensWithHighRates {
					rates := ratesByToken[token]

					// Filter rates based on user preferences
					for _, rate := range rates {
						if !shouldShowCEXRates(chatID) && rate.Category == "CEX" {
							continue
						}
					}

					message.WriteString(fmt.Sprintf("🪙 *%s*\n", token))

					// Sort rates by source for consistent ordering
					sort.Slice(rates, func(i, j int) bool {
						return rates[i].Source < rates[j].Source
					})

					for _, rate := range rates {
						message.WriteString(formatRate(rate, lendingThresholds[token]))
						message.WriteString("\n")
					}
					message.WriteString("\n")
				}

				msg := tgbotapi.NewMessage(chatID, message.String())
				msg.ParseMode = "markdown"
				sendTelegramMessage(bot, msg)
			}
		} else {
			// Log rates that were checked but didn't meet the significance threshold
			log.Println("No rates met the significance threshold")
		}
	}

	// Perform initial fetch
	log.Println("Performing initial rate fetch...")
	cronFetchRates()

	// Start a goroutine to handle rate checking and notifications
	c := cron.New()

	// Schedule rate fetching for the 59th minute of every hour
	_, err = c.AddFunc("*/2 * * * *", cronFetchRates)

	if err != nil {
		log.Fatal("Error setting up cron job:", err)
	}

	// Start the cron scheduler
	c.Start()

	// Set bot commands for menu
	commands := make([]tgbotapi.BotCommand, 0, len(commandHelp))
	for cmd, desc := range commandHelp {
		// Remove the first line of description for menu (keep it short)
		shortDesc := strings.Split(desc, "\n")[0]
		commands = append(commands, tgbotapi.BotCommand{
			Command:     cmd[1:], // Remove leading slash
			Description: shortDesc,
		})
	}

	// Sort commands for consistent ordering
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Command < commands[j].Command
	})

	_, err = bot.Request(tgbotapi.NewSetMyCommands(commands...))
	if err != nil {
		log.Printf("Error setting bot commands: %v", err)
	}

	// Handle incoming messages
	for update := range updates {
		if update.Message == nil {
			continue
		}

		// Handle different commands
		switch {
		case update.Message.Text == "/start":
			err := db.AddSubscriber(update.Message.Chat.ID)
			if err != nil {
				log.Printf("Error adding subscriber: %v", err)
				msg := tgbotapi.NewMessage(update.Message.Chat.ID,
					"Sorry, there was an error processing your request. Please try again later.")
				sendTelegramMessage(bot, msg)
				continue
			}
			activeChatIDs[update.Message.Chat.ID] = true
			msg := tgbotapi.NewMessage(update.Message.Chat.ID,
				"Welcome! You will now receive notifications when lending rates exceed thresholds.")
			sendTelegramMessage(bot, msg)

		case update.Message.Text == "/stop":
			err := db.RemoveSubscriber(update.Message.Chat.ID)
			if err != nil {
				log.Printf("Error removing subscriber: %v", err)
				msg := tgbotapi.NewMessage(update.Message.Chat.ID,
					"Sorry, there was an error processing your request. Please try again later.")
				sendTelegramMessage(bot, msg)
				continue
			}
			delete(activeChatIDs, update.Message.Chat.ID)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID,
				"You have been unsubscribed from notifications.")
			sendTelegramMessage(bot, msg)

		case strings.HasPrefix(update.Message.Text, "/rate"):
			// Use cached rates or fetch new ones
			allRates, err := getRatesWithCache(sources...)
			if err != nil {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID,
					"Error fetching rates. Please try again later.")
				sendTelegramMessage(bot, msg)
				continue
			}

			if len(allRates) == 0 {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID,
					"No rates available yet. Please try again in a few minutes.")
				sendTelegramMessage(bot, msg)
				continue
			}

			// Parse the token from command (e.g., "/rate USDT" or just "/rate")
			parts := strings.Fields(update.Message.Text)
			var message strings.Builder

			if len(parts) > 1 {
				// Query specific token
				token := strings.ToUpper(parts[1])
				message.WriteString(fmt.Sprintf("*Current Rates for %s*\n", token))

				found := false
				tokenRates := []Rate{}
				for _, rate := range allRates {
					if rate.Token == token {
						if rate.Category == "CEX" && !shouldShowCEXRates(update.Message.Chat.ID) {
							continue
						}
						tokenRates = append(tokenRates, rate)
						found = true
					}
				}

				if !found {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID,
						fmt.Sprintf("No rates found for token: %s", token))
					sendTelegramMessage(bot, msg)
					continue
				}

				// Sort rates by source
				sort.Slice(tokenRates, func(i, j int) bool {
					return tokenRates[i].Source < tokenRates[j].Source
				})

				threshold := lendingThresholds[token]
				for _, rate := range tokenRates {
					message.WriteString(formatRate(rate, threshold))
					message.WriteString("\n")
				}

			} else {
				// Show all rates
				message.WriteString("*Current Rates for All Tokens*\n")

				// Group rates by token
				ratesByToken := make(map[string][]Rate)
				for _, rate := range allRates {
					ratesByToken[rate.Token] = append(ratesByToken[rate.Token], rate)
				}

				// Sort tokens for consistent ordering
				var tokens []string
				for token := range ratesByToken {
					tokens = append(tokens, token)
				}
				sort.Strings(tokens)

				for _, token := range tokens {
					rates := ratesByToken[token]
					message.WriteString(fmt.Sprintf("🪙 *%s*\n", token))

					// Sort rates by source
					sort.Slice(rates, func(i, j int) bool {
						return rates[i].Source < rates[j].Source
					})

					threshold := lendingThresholds[token]
					for _, rate := range rates {
						message.WriteString(formatRate(rate, threshold))
						message.WriteString("\n")
					}
					message.WriteString("\n")
				}
			}

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, message.String())
			msg.ParseMode = "markdown"
			sendTelegramMessage(bot, msg)

		case update.Message.Text == "/help":
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, getHelpMessage())
			msg.ParseMode = "markdown"
			sendTelegramMessage(bot, msg)

		case update.Message.Text == "/cex":
			newValue := !shouldShowCEXRates(update.Message.Chat.ID)
			err := db.SetShowCEX(update.Message.Chat.ID, newValue)
			if err != nil {
				log.Printf("Error saving CEX preference: %v", err)
				msg := tgbotapi.NewMessage(update.Message.Chat.ID,
					"Sorry, there was an error saving your preference. Please try again later.")
				sendTelegramMessage(bot, msg)
				continue
			}
			userPreferences[update.Message.Chat.ID] = newValue
			status := "enabled"
			if !newValue {
				status = "disabled"
			}
			msg := tgbotapi.NewMessage(update.Message.Chat.ID,
				fmt.Sprintf("CEX rates are now %s.", status))
			sendTelegramMessage(bot, msg)

		default:
			if update.Message.Text != "" {
				log.Printf("Unknown command: %s", update.Message.Text)
			}
		}
	}
}

func sendTelegramMessage(bot *tgbotapi.BotAPI, msg tgbotapi.MessageConfig) {
	_, err := bot.Send(msg)
	if err != nil {
		log.Printf("Error sending Telegram message: %v, msg: %+v", err, msg)
	}
}

func joinStrings(strings []string, separator string) string {
	result := ""
	for i, s := range strings {
		if i > 0 {
			result += separator
		}
		result += s
	}
	return result
}

// getEnv retrieves the value of the environment variable named by the key.
// It returns the value, which will be empty if the variable is not present.
// If the variable is not present and a default value is given, it returns the default value.
func getEnv(key, defaultValue string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	return value
}

func formatRate(rate Rate, threshold float64) string {
	var lendingRateStr, borrowRateStr string

	// Format lending rate
	if rate.LendingRate >= threshold*2 {
		lendingRateStr = fmt.Sprintf("🔥%.0f%%", rate.LendingRate)
	} else if rate.LendingRate >= threshold {
		lendingRateStr = fmt.Sprintf("*%.0f%%*", rate.LendingRate)
	} else {
		lendingRateStr = fmt.Sprintf("%.0f%%", rate.LendingRate)
	}

	// Format borrow rate (optional: you might want to add threshold for borrow rates too)
	borrowRateStr = fmt.Sprintf("%.0f%%", rate.BorrowRate)

	return fmt.Sprintf("  • %s: L: %s | B: %s",
		rate.Source, lendingRateStr, borrowRateStr)
}

// convertAPRtoAPY converts APR to APY
// compounds is the number of times interest is compounded per year
func convertAPRtoAPY(apr float64, compounds int) float64 {
	// Convert percentage to decimal
	aprDecimal := apr / 100

	// Calculate APY
	apy := math.Pow(1+aprDecimal/float64(compounds), float64(compounds)) - 1

	// Convert back to percentage
	return apy * 100
}
