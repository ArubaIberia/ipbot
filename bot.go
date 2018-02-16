package main

import (
	"fmt"
	"log"
	"strings"

	"gopkg.in/telegram-bot-api.v4"
)

// Tokens in a message
type Tokens struct {
	tokens  []string
	current int
}

// TokensFrom gets all tokens from message
func TokensFrom(msg *tgbotapi.Message) *Tokens {
	return &Tokens{tokens: strings.Fields(msg.Text)}
}

// Next token in a message
func (tokens *Tokens) Next() string {
	counter := tokens.current
	if counter >= len(tokens.tokens) {
		return ""
	}
	tokens.current++
	return tokens.tokens[counter]
}

// Back one token
func (tokens *Tokens) Back() {
	if tokens.current > 0 {
		tokens.current--
	}
}

// Remaining tokens
func (tokens *Tokens) Remaining() int {
	return len(tokens.tokens) - tokens.current
}

// ReplyFunc models a function that can reply an order.
type ReplyFunc func(bot Bot, msg *tgbotapi.Message, tokens *Tokens) string

// Bot that manages the connection to Telegram
type Bot interface {
	// Add a reply func
	Add(key string, reply ReplyFunc)
	// Add a master
	AddMaster(master string)
	// Loop through the messages
	Loop()
}

type bot struct {
	api     *tgbotapi.BotAPI
	replies map[string]ReplyFunc
	masters []string
	pin     string
}

// NewBot creates a new Telegram Bot
func NewBot(token string) (Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	api.Debug = true
	log.Printf("Bot username %s", api.Self.UserName)
	result := &bot{
		masters: make([]string, 0, 10),
		api:     api,
		replies: make(map[string]ReplyFunc),
	}
	result.Add("master", func(bot Bot, msg *tgbotapi.Message, tokens *Tokens) string {
		if tokens.Remaining() <= 0 {
			return "You must specify the PIN"
		}
		master := tokens.Next()
		bot.AddMaster(master)
		return fmt.Sprintf("Username %s added as master", master)
	})
	return result, nil
}

// Checks if the message comes from some of the masters
func (bot *bot) isMaster(msg *tgbotapi.Message) bool {
	uname := msg.From.String()
	// First message becomes master
	if bot.masters == nil || len(bot.masters) == 0 {
		bot.masters = append(bot.masters, uname)
		bot.api.Send(tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("%s has become my first master", uname)))
		return true
	}
	// Look in master list
	found := false
	for _, master := range bot.masters {
		if master == uname {
			found = true
			break
		}
	}
	if !found {
		bot.api.Send(tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("%s is not my master", uname)))
	}
	return found
}

func (bot *bot) Add(key string, reply ReplyFunc) {
	bot.replies[strings.ToLower(key)] = reply
}

func (bot *bot) AddMaster(master string) {
	bot.masters = append(bot.masters, master)
}

// Messages coming from the robot
func (bot *bot) messages() <-chan *tgbotapi.Message {
	// Update channel
	result := make(chan *tgbotapi.Message)
	go func(result chan *tgbotapi.Message) {
		defer close(result)
		updates := tgbotapi.NewUpdate(0)
		updates.Timeout = 60
		items, err := bot.api.GetUpdatesChan(updates)
		if err != nil {
			log.Print("Error: ", err)
			return
		}
		for item := range items {
			// Message can be new or edited
			message := item.Message
			if message == nil {
				message = item.EditedMessage
			}
			if message != nil && bot.isMaster(message) {
				result <- message
			}
		}
	}(result)
	return result
}

// Help string
func (bot *bot) help() string {
	// Full list of handlers
	orderlist := make([]string, 0, len(bot.replies))
	for name := range bot.replies {
		orderlist = append(orderlist, name)
	}
	return fmt.Sprintf("Known commands:\n  - %s", strings.Join(orderlist, "\n  - "))
}

func (bot *bot) Loop() {
	for msg := range bot.messages() {
		// Start parsing tokens
		tokens := TokensFrom(msg)
		for tokens.Remaining() > 0 {
			result := ""
			order := strings.ToLower(tokens.Next())
			if reply, ok := bot.replies[order]; !ok {
				result = fmt.Sprintf("Command %s is not known.\n%s", order, bot.help())
			} else {
				remaining := tokens.Remaining()
				result = reply(bot, msg, tokens)
				if tokens.Remaining() > remaining {
					result = fmt.Sprintf("Possible loop in command %s, len(remainder) has grown", order)
					bot.api.Send(tgbotapi.NewMessage(msg.Chat.ID, result))
					return
				}
			}
			newMsg := tgbotapi.NewMessage(msg.Chat.ID, result)
			//newMsg.ReplyToMessageID = update.Message.MessageID
			bot.api.Send(newMsg)
		}
	}
}
