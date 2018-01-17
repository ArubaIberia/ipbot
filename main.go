package main

import (
	"flag"
	"log"
	"os"
	"strings"
	"time"
)

func main() {

	// Get API key from command line
	apiKey := ""
	cmdKey := flag.String("token", "", "Telegram API token")
	flag.Parse()
	if cmdKey != nil && *cmdKey != "" {
		apiKey = *cmdKey
	} else {
		// Get API Key from environment
		for _, key := range os.Environ() {
			if strings.HasPrefix(key, "IPBOT_API_KEY=") {
				apiKey = strings.SplitN(key, "=", 2)[1]
				break
			}
		}
	}
	if apiKey == "" {
		log.Fatal("You must provide Telegram token (-token <telegram token> or environment variable IPBOT_API_KEY)")
	}

	for {
		// Build the bot
		bot, err := NewBot(apiKey)
		if err != nil {
			log.Fatal("Error creating bot: ", err)
		}
		// Register handlers
		interfaces := RegisterIP(bot)
		RegisterVLAN(bot, interfaces)
		// Loop
		bot.Loop()
		log.Print("Loop exited, retrying in five minutes...")
		time.Sleep(5 * time.Minute)
	}
}
