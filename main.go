package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"gopkg.in/telegram-bot-api.v4"
)

type ReplyFunc func(msg *tgbotapi.Message) string

func getHandlers() map[string]ReplyFunc {
	return map[string]ReplyFunc{
		"ip": replyToIP,
	}
}

func main() {

	token := flag.String("token", "", "Telegram API token")
	flag.Parse()
	if token == nil || *token == "" {
		log.Fatal("Debe especificar el token Telegram con el parametro -token")
	}

	for {
		if err := loop(*token); err != nil {
			log.Print("Error: ", err, "\nEsperando 5 minutos...")
			time.Sleep(5 * time.Minute)
		}
	}
}

func loop(token string) error {

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return err
	}

	bot.Debug = true
	log.Printf("Authorizado en la cuenta %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	handlers := getHandlers()
	orderlist := make([]string, 0, len(handlers))

	for name := range handlers {
		orderlist = append(orderlist, name)
	}
	orders := strings.Join(orderlist, "\n  - ")

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

		reply := ""
		for command, handler := range handlers {
			if strings.EqualFold(strings.TrimSpace(update.Message.Text), command) {
				reply = handler(update.Message)
			}
		}
		if reply == "" {
			reply = fmt.Sprintf("Orden %s no reconocida. Ordenes reconocidas:\n  - %s", update.Message.Text, orders)
		}

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, reply)
		//msg.ReplyToMessageID = update.Message.MessageID
		bot.Send(msg)
	}

	return nil
}
