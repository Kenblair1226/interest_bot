package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

const checkInterval = 30 * time.Minute

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

	// Load lending rate thresholds from environment variables
	lendingThresholdUSDCStr := getEnv("LENDING_THRESHOLD_USDC", "30") // Default threshold for USDC is 10%
	lendingThresholdTIAStr := getEnv("LENDING_THRESHOLD_TIA", "30")   // Default threshold for TIA is 30%
	lendingThresholdUSDTStr := getEnv("LENDING_THRESHOLD_USDT", "30") // Default threshold for USDT is 15%

	lendingThresholdUSDC, err := strconv.ParseFloat(lendingThresholdUSDCStr, 64)
	if err != nil {
		log.Fatal("Invalid LENDING_THRESHOLD_USDC:", err)
	}

	lendingThresholdTIA, err := strconv.ParseFloat(lendingThresholdTIAStr, 64)
	if err != nil {
		log.Fatal("Invalid LENDING_THRESHOLD_TIA:", err)
	}

	lendingThresholdUSDT, err := strconv.ParseFloat(lendingThresholdUSDTStr, 64)
	if err != nil {
		log.Fatal("Invalid LENDING_THRESHOLD_USDT:", err)
	}

	for {
		okxUpdates, err := okxSource.FetchRates()
		if err != nil {
			log.Printf("Error fetching OKX rates: %v", err)
		}

		neptuneUpdates, err := neptuneSource.FetchRates()
		if err != nil {
			log.Printf("Error fetching Neptune rates: %v", err)
		}

		allUpdates := append(okxUpdates, neptuneUpdates...)

		// Filter updates based on lending rate thresholds
		filteredUpdates := []string{}
		for _, update := range allUpdates {
			if strings.Contains(update, "USDC") {
				// Extract the lending rate from the update string
				lendingRateStr := extractLendingRate(update, "USDC")
				if lendingRateStr == "" {
					continue
				}
				lendingRate, err := strconv.ParseFloat(lendingRateStr, 64)
				if err != nil {
					log.Printf("Error parsing lending rate for USDC: %v", err)
					continue
				}
				if lendingRate >= lendingThresholdUSDC {
					filteredUpdates = append(filteredUpdates, update)
				}
			} else if strings.Contains(update, "TIA") {
				// Extract the lending rate from the update string
				lendingRateStr := extractLendingRate(update, "TIA")
				if lendingRateStr == "" {
					continue
				}
				lendingRate, err := strconv.ParseFloat(lendingRateStr, 64)
				if err != nil {
					log.Printf("Error parsing lending rate for TIA: %v", err)
					continue
				}
				if lendingRate >= lendingThresholdTIA {
					filteredUpdates = append(filteredUpdates, update)
				}
			} else if strings.Contains(update, "USDT") {
				// Extract the lending rate from the update string
				lendingRateStr := extractLendingRate(update, "USDT")
				if lendingRateStr == "" {
					continue
				}
				lendingRate, err := strconv.ParseFloat(lendingRateStr, 64)
				if err != nil {
					log.Printf("Error parsing lending rate for USDT: %v", err)
					continue
				}
				if lendingRate >= lendingThresholdUSDT {
					filteredUpdates = append(filteredUpdates, update)
				}
			}
		}

		if len(filteredUpdates) > 0 {
			message := "High lending rate updates:\n" + joinStrings(filteredUpdates, "\n")
			sendTelegramMessage(bot, chatID, message)
		}

		time.Sleep(checkInterval)
	}
}

func sendTelegramMessage(bot *tgbotapi.BotAPI, chatID int64, message string) {
	msg := tgbotapi.NewMessage(chatID, message)
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
