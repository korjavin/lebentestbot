package config

import (
	"errors"
	"os"
)

// Config holds all the configuration for the application
type Config struct {
	BotToken       string
	DeepseekAPIKey string
	DatabasePath   string
}

// Load loads the configuration from environment variables
func Load() (*Config, error) {
	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		return nil, errors.New("BOT_TOKEN environment variable is required")
	}

	deepseekAPIKey := os.Getenv("DEEPSEEK_API_KEY")
	if deepseekAPIKey == "" {
		return nil, errors.New("DEEPSEEK_API_KEY environment variable is required")
	}

	// Set database path with default
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/lebentest.db"
	}

	return &Config{
		BotToken:       botToken,
		DeepseekAPIKey: deepseekAPIKey,
		DatabasePath:   dbPath,
	}, nil
}
