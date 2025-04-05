# LebenTestBot

A Telegram bot designed to help users practice for German tests using real test questions.

## Features

- 🎲 Random questions from a test question database
- 🖼️ Support for questions with images
- 🤖 AI-powered explanations using the Deepseek API
- 📊 User statistics tracking
- 💾 Response caching to minimize API calls
- 🔍 Detailed help and analysis for each question

## Commands

- `/start` - Start the bot and get a random question
- `/next` - Get another random question
- `/help` - Get AI-powered assistance with the current question
- `/stat` - View your statistics

## Setup and Installation

### Prerequisites

- Go 1.18 or higher
- Telegram Bot Token (from [@BotFather](https://t.me/BotFather))
- Deepseek API Key 

### Running Locally

1. Clone this repository:
```
git clone https://github.com/korjavin/lebentestbot.git
cd lebentestbot
```

2. Set environment variables:
```bash
export BOT_TOKEN="your_telegram_bot_token"
export DEEPSEEK_API_KEY="your_deepseek_api_key"
export DB_PATH="./data/lebentest.db" # Optional, defaults to this value
```

3. Build and run:
```bash
go build -o lebentestbot
./lebentestbot
```

### Using Docker

1. Build the Docker image:
```bash
docker build -t lebentestbot .
```

2. Run the Docker container:
```bash
docker run -d \
  -e BOT_TOKEN="your_telegram_bot_token" \
  -e DEEPSEEK_API_KEY="your_deepseek_api_key" \
  -v "$(pwd)/data:/app/data" \
  --name lebentestbot \
  lebentestbot
```

### Using Prebuilt Image

You can also use the prebuilt image from GitHub Container Registry:

```bash
docker pull ghcr.io/korjavin/lebentestbot:latest

docker run -d \
  -e BOT_TOKEN="your_telegram_bot_token" \
  -e DEEPSEEK_API_KEY="your_deepseek_api_key" \
  -v "$(pwd)/data:/app/data" \
  --name lebentestbot \
  ghcr.io/korjavin/lebentestbot:latest
```

## Project Structure

```
lebentestbot/
├── ai/              # AI integration with Deepseek
├── assets/
│   ├── images/      # Question images
│   └── questions.json  # Questions database
├── bot/             # Core bot functionality
├── config/          # Configuration handling
├── database/        # Database operations
├── models/          # Data models
├── .github/workflows/ # GitHub Actions workflows
├── Dockerfile       # Container definition
├── README.md        # This file
├── go.mod           # Go module definition
└── main.go          # Application entry point
```

## Database

The bot uses SQLite for persistence, storing:
- User activity (questions answered)
- AI response cache (to avoid duplicate API calls)
- Correct answers determined by AI

## Development

To contribute to this project:

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Commit your changes: `git commit -m 'Add amazing feature'`
4. Push to your branch: `git push origin feature/amazing-feature`
5. Open a Pull Request

## License

MIT