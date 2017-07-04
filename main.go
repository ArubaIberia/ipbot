package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"gopkg.in/telegram-bot-api.v4"
)

type ReplyFunc func(msg *tgbotapi.Message, fields []string) (string, []string)

func getHandlers() map[string]ReplyFunc {
	interfaces := &Interfaces{}
	vlans := &VLAN{Selected: 0, Interfaces: interfaces}
	interfaces.Update()
	return map[string]ReplyFunc{
		"ip": func(msg *tgbotapi.Message, fields []string) (string, []string) {
			return interfaces.ReplyToIP(msg, fields)
		},
		"vlan": func(msg *tgbotapi.Message, fields []string) (string, []string) {
			return vlans.ReplyToVLAN(msg, fields)
		},
		"out": func(msg *tgbotapi.Message, fields []string) (string, []string) {
			return vlans.ReplyToOut(msg, fields)
		},
		"in": func(msg *tgbotapi.Message, fields []string) (string, []string) {
			return vlans.ReplyToIn(msg, fields)
		},
	}
}

func main() {

	token := flag.String("token", "", "Telegram API token")
	flag.Parse()
	if token == nil || *token == "" {
		log.Fatal("You must provide Telegram token (-token <telegram token>)")
	}

	for {
		if err := loop(*token); err != nil {
			log.Print("Error: ", err, "\nRetrying in five minutes...")
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
	log.Printf("Bot username %s", bot.Self.UserName)

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
		fields := strings.Fields(update.Message.Text)

		// Can take several orders in a single line
		for len(fields) > 0 {
			log.Printf("[%s] %s", update.Message.From.UserName, strings.Join(fields, " "))
			reply := ""
			remainder := []string{}
			for command, handler := range handlers {
				if strings.EqualFold(fields[0], command) {
					reply, remainder = handler(update.Message, fields)
					break
				}
			}
			if reply == "" {
				reply = fmt.Sprintf("Command %s is not known.\nKnown commands:\n  - %s", update.Message.Text, orders)
				remainder = nil
			}
			if remainder != nil && len(remainder) > 0 {
				if len(remainder) < len(fields) {
					fields = remainder
				} else {
					reply = strings.Join([]string{
						reply,
						fmt.Sprintf("Possible loop in command %s, len(remainder) >= len(fields)", remainder[0]),
					}, "\n")
					fields = nil
				}
			} else {
				fields = nil
			}

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, reply)
			//msg.ReplyToMessageID = update.Message.MessageID
			bot.Send(msg)
		}
	}

	return nil
}
