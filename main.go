package main

import (
	"log"
	"os"

	"github.com/korjavin/lebentestbot/bot"
	"github.com/korjavin/lebentestbot/config"
)

func main() {
	// Configure logging
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("Starting LebenTestBot...")

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize and start the bot
	b, err := bot.New(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize bot: %v", err)
	}

	log.Println("Bot initialized successfully")
	b.Start()
}
