package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

const checkInterval = 30 * time.Minute

// // Define a unified Rate struct
// type Rate struct {
// 	Source      string  `json:"source"`
// 	Token       string  `json:"token"`
// 	BorrowRate  float64 `json:"borrow_rate"`
// 	LendingRate float64 `json:"lending_rate"`
// }

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

	// Get Chat ID
	chatIDStr := getEnv("CHAT_ID", "")
	if chatIDStr == "" {
		log.Fatal("CHAT_ID is not set")
	}

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		log.Fatal("Error parsing CHAT_ID:", err)
	}

	bot, err := tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		log.Panic(err)
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	okxSource := NewOKXSource()
	neptuneSource := NewNeptuneSource()

	// Load lending rate thresholds into a map for scalability
	lendingThresholds := map[string]float64{
		"USDC": 30.0,
		"TIA":  30.0,
		"USDT": 30.0,
	}

	// Optionally, load thresholds from environment variables
	for token, defaultThreshold := range lendingThresholds {
		thresholdStr := getEnv(fmt.Sprintf("LENDING_THRESHOLD_%s", token), fmt.Sprintf("%.0f", defaultThreshold))
		threshold, err := strconv.ParseFloat(thresholdStr, 64)
		if err != nil {
			log.Fatalf("Invalid LENDING_THRESHOLD_%s: %v", token, err)
		}
		lendingThresholds[token] = threshold
	}

	injeraSource := NewInjeraSource() // Initialize Injera source

	for {
		okxRates, err := okxSource.FetchRates()
		if err != nil {
			log.Printf("Error fetching OKX rates: %v", err)
		}

		neptuneRates, err := neptuneSource.FetchRates()
		if err != nil {
			log.Printf("Error fetching Neptune rates: %v", err)
		}

		injeraRates, err := injeraSource.FetchRates()
		if err != nil {
			log.Printf("Error fetching Injera rates: %v", err)
		}

		allRates := append(okxRates, neptuneRates...)
		allRates = append(allRates, injeraRates...)

		log.Printf("Fetched rates: %+v", allRates)

		// Filter rates based on lending rate thresholds
		filteredRates := []Rate{}
		for _, rate := range allRates {
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
			for _, rate := range allRates {
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

				for _, rate := range rates {
					message.WriteString(fmt.Sprintf("  • %s:\n", rate.Source))
					message.WriteString(fmt.Sprintf("    └ Lend: `%.2f%%`\n", rate.LendingRate))
					message.WriteString(fmt.Sprintf("    └ Borrow: `%.2f%%`\n", rate.BorrowRate))
				}
				message.WriteString("\n")
			}

			msg := tgbotapi.NewMessage(chatID, message.String())
			msg.ParseMode = "markdown"
			sendTelegramMessage(bot, chatID, msg)
		}

		time.Sleep(checkInterval)
	}
}

func sendTelegramMessage(bot *tgbotapi.BotAPI, chatID int64, msg tgbotapi.MessageConfig) {
	_, err := bot.Send(msg)
	if err != nil {
		log.Printf("Error sending Telegram message: %v", err)
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

// extractLendingRate extracts the lending rate from the update string for the given token
func extractLendingRate(update, token string) string {
	// Example update format: "Neptune USDC: Lend: 12.34%, Borrow: 5.67%"
	prefix := fmt.Sprintf("%s: Lend: ", token)
	start := strings.Index(update, prefix)
	if start == -1 {
		return ""
	}
	start += len(prefix)
	end := strings.Index(update[start:], "%")
	if end == -1 {
		return ""
	}
	return update[start : start+end]
}
