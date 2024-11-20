package main

import (
	"fmt"
	"log"
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
	db *Database
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
	var allRates []Rate
	var errors []error

	for _, source := range sources {
		rates, err := source.FetchRates()
		if err != nil {
			log.Printf("Error fetching rates from %T: %v", source, err)
			errors = append(errors, err)
			continue
		}
		allRates = append(allRates, rates...)
	}

	if len(allRates) == 0 && len(errors) > 0 {
		return nil, fmt.Errorf("all sources failed to fetch rates")
	}

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
	sources := []RateSource{okxSource, neptuneSource, injeraSource, binanceSource}

	// Function to fetch and process rates
	cronFetchRates := func() {
		rates, err := fetchRates(sources...)
		if err != nil {
			log.Printf("Error fetching rates: %v", err)
			return
		}

		// Filter rates based on lending rate thresholds
		filteredRates := []Rate{}
		for _, rate := range rates {
			threshold, exists := lendingThresholds[rate.Token]
			if !exists {
				continue
			}
			if rate.LendingRate >= threshold {
				filteredRates = append(filteredRates, rate)
			}
		}

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

			var message strings.Builder
			message.WriteString("*High Lending Rate Updates*\n\n")

			for token := range tokensWithHighRates {
				rates := ratesByToken[token]
				message.WriteString(fmt.Sprintf("🪙 *%s*\n", token))

				// Sort rates by source for consistent ordering
				sort.Slice(rates, func(i, j int) bool {
					return rates[i].Source < rates[j].Source
				})

				threshold := lendingThresholds[token]
				// Only show rates that are above 10%
				for _, rate := range rates {
					if rate.LendingRate >= 10.0 {
						message.WriteString(formatRate(rate, threshold))
						message.WriteString("\n")
					}
				}
				message.WriteString("\n")
			}

			// Send notification to all active chat IDs
			msg := tgbotapi.NewMessage(0, message.String())
			msg.ParseMode = "markdown"
			for chatID := range activeChatIDs {
				msg.ChatID = chatID
				sendTelegramMessage(bot, msg)
			}
		}
	}

	// Perform initial fetch
	log.Println("Performing initial rate fetch...")
	cronFetchRates()

	// Start a goroutine to handle rate checking and notifications
	c := cron.New()

	// Schedule rate fetching for the 59th minute of every hour
	_, err = c.AddFunc("59 * * * *", cronFetchRates)

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
				message.WriteString(fmt.Sprintf("*Current Rates for %s*\n\n", token))

				found := false
				tokenRates := []Rate{}
				for _, rate := range allRates {
					if rate.Token == token {
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
				message.WriteString("*Current Rates for All Tokens*\n\n")

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
