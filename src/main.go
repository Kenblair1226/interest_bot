package main

import (
	"log"
	"os"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

const checkInterval = 15 * time.Minute

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

		if len(allUpdates) > 0 {
			message := "Interest rate updates:\n" + joinStrings(allUpdates, "\n")
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
