# Interest Rate Bot

This Telegram bot monitors interest rates from OKX and Neptune platforms and sends notifications when rates change.

## Prerequisites

- Docker and Docker Compose

## Setup

1. Clone the repository:
   ```
   git clone <your-repository-url>
   cd <repository-directory>
   ```

2. Create a `.env` file in the root directory with the following content:
   ```
   TELEGRAM_TOKEN=your_telegram_bot_token
   CHAT_ID=your_telegram_chat_id
   ```
   Replace `your_telegram_bot_token` and `your_telegram_chat_id` with your actual Telegram bot token and chat ID.

## Running the Application with Docker

To run the application using Docker, use the following command:

```
docker-compose up --build
```

This command will build the Docker image and start the container. The bot will start running and will check for interest rate updates every 15 minutes.

## Stopping the Application

To stop the application, use:

```
docker-compose down
```

## Notes

- The bot checks for rate updates every 15 minutes.
- Make sure your `.env` file is properly configured before running the Docker container.
- The `.env` file is used by Docker Compose to set environment variables in the container. It should never be committed to version control.
