# OKX Interest Rate Bot

This Telegram bot monitors the interest rate for TIA (Celestia) on OKX and sends notifications when the rate changes.

## Prerequisites

- Go 1.20 or later
- A Telegram Bot Token
- Your Telegram Chat ID

## Setup

1. Clone the repository:   ```
   git clone <your-repository-url>
   cd <repository-directory>   ```

2. Install dependencies:   ```
   go mod tidy   ```

3. Create a `.env` file in the root directory with the following content:   ```
   TELEGRAM_TOKEN=your_telegram_bot_token
   CHAT_ID=your_telegram_chat_id   ```
   Replace `your_telegram_bot_token` and `your_telegram_chat_id` with your actual Telegram bot token and chat ID.

## Running the Application

To run the application, use the following command from the root directory of the project:
```
go run src/okx_interest_bot.go
```

The bot will start running and will check for interest rate updates every 15 minutes. It will send a message to your specified Telegram chat when the estimated rate for the next hour changes.

## Building the Application

If you want to build the application into an executable, use:
```
go build -o interest_bot src/okx_interest_bot.go
```

This will create an executable named `interest_bot` in your current directory. You can then run it with:
```
./interest_bot
```

## Notes

- The bot checks for rate updates every 15 minutes.
- Make sure your `.env` file is properly configured and in the same directory as the executable when running the built version.
- The `.env` file is ignored by git for security reasons. Never commit your actual token or chat ID to version control.