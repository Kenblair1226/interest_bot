package main

import (
	"log"
	"os"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const checkInterval = 15 * time.Minute

func main() {
	telegramToken := os.Getenv("TELEGRAM_TOKEN")
	if telegramToken == "" {
		log.Fatal("TELEGRAM_TOKEN environment variable is not set")
	}

	chatIDStr := os.Getenv("CHAT_ID")
	if chatIDStr == "" {
		log.Fatal("CHAT_ID environment variable is not set")
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
