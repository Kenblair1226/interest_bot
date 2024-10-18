package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

const (
	okxAPIURL     = "https://www.okx.com/priapi/v2/financial/market-lending-info?currencyId=2854"
	checkInterval = 15 * time.Minute
)

type OKXResponse struct {
	Data struct {
		List []struct {
			CurrencyName  string  `json:"currencyName"`
			EstimatedRate float64 `json:"estimatedRate"`
			PreRate       float64 `json:"preRate"`
			AvgRate       float64 `json:"avgRate"`
		} `json:"list"`
	} `json:"data"`
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	telegramToken := os.Getenv("TELEGRAM_TOKEN")
	chatIDStr := os.Getenv("CHAT_ID")
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		log.Fatal("Error parsing CHAT_ID:", err)
	}

	bot, err := tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		log.Panic(err)
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	var lastEstimatedRate float64

	for {
		estimatedRate, preRate, avgRate, err := fetchInterestRates()
		if err != nil {
			log.Printf("Error fetching interest rates: %v", err)
			time.Sleep(checkInterval)
			continue
		}

		if estimatedRate != lastEstimatedRate {
			message := fmt.Sprintf("TIA interest rate update:\nEstimated next hour: %.2f%%\nPrevious hour: %.2f%%\nAverage rate: %.2f%%",
				estimatedRate*100, preRate*100, avgRate*100)
			sendTelegramMessage(bot, chatID, message)
			lastEstimatedRate = estimatedRate
		}

		time.Sleep(checkInterval)
	}
}

func fetchInterestRates() (float64, float64, float64, error) {
	resp, err := http.Get(okxAPIURL)
	if err != nil {
		return 0, 0, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, 0, err
	}

	var okxResp OKXResponse
	err = json.Unmarshal(body, &okxResp)
	if err != nil {
		return 0, 0, 0, err
	}

	if len(okxResp.Data.List) == 0 {
		return 0, 0, 0, fmt.Errorf("no data found in OKX response")
	}

	return okxResp.Data.List[0].EstimatedRate, okxResp.Data.List[0].PreRate, okxResp.Data.List[0].AvgRate, nil
}

func sendTelegramMessage(bot *tgbotapi.BotAPI, chatID int64, message string) {
	msg := tgbotapi.NewMessage(chatID, message)
	_, err := bot.Send(msg)
	if err != nil {
		log.Printf("Error sending Telegram message: %v", err)
	}
}
