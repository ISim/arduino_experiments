package telegram

import (
	tba "github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"os"
)

var (
	botAPI *tba.BotAPI
)

func init() {
	var err error

	if token := os.Getenv(EnvBotToken); token != "" {
		botAPI, err = tba.NewBotAPI(os.Getenv(EnvBotToken))
		if err != nil {
			log.Fatalf("bot API intialization error: %s", err.Error())
		}
	}
}
