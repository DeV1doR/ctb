package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"time"

	tgbotapi "github.com/Syfaro/telegram-bot-api"
)

type CoinmarketDict struct {
	Currency string `json:"symbol"`
	Price    string `json:"price_usd"`
}

var (
	// seconds
	sendUpdateEvery    = 500
	coinmarketLastData = map[string]map[string]float64{
		"BTC": map[string]float64{
			"last":    0,
			"current": 0,
		},
		"ETH": map[string]float64{
			"last":    0,
			"current": 0,
		},
		"ETC": map[string]float64{
			"last":    0,
			"current": 0,
		},
	}
	httpClient = &http.Client{Timeout: 10 * time.Second}
	botToken   = flag.String("token", "", "telegram bot token")
)

func getJson(url string, target interface{}) error {
	r, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(target)
}

func updateCoimarketInfo() error {
	data := make([]CoinmarketDict, 0)
	if err := getJson("https://api.coinmarketcap.com/v1/ticker/?limit=10", &data); err != nil {
		return err
	}
	for _, el := range data {
		if _, ok := coinmarketLastData[el.Currency]; !ok {
			continue
		}
		price, err := strconv.ParseFloat(el.Price, 64)
		if err != nil {
			return err
		}
		if coinmarketLastData[el.Currency]["last"] != 0 {
			coinmarketLastData[el.Currency]["last"] = coinmarketLastData[el.Currency]["current"]
		} else {
			coinmarketLastData[el.Currency]["last"] = price
		}
		coinmarketLastData[el.Currency]["current"] = price

		log.Printf("%+v \n", coinmarketLastData)
	}
	return nil
}

func main() {
	flag.Parse()

	bot, err := tgbotapi.NewBotAPI(*botToken)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	// инициализируем канал, куда будут прилетать обновления от API
	ucfg := tgbotapi.NewUpdate(0)
	ucfg.Timeout = 60

	var updates tgbotapi.UpdatesChannel
	updates, err = bot.GetUpdatesChan(ucfg)

	currentUsers := make(map[int64]bool)

	log.Printf("%+v \n", coinmarketLastData)

	ticker := time.NewTicker(time.Second * time.Duration(sendUpdateEvery))
	doneTicker := make(chan struct{})

	go func() {
		for {
			if err := updateCoimarketInfo(); err != nil {
				log.Printf("Coinmarket update error: %s \n", err)
			}

			select {
			case <-ticker.C:
				for UID := range currentUsers {
					var buffer bytes.Buffer
					var keys []string
					for k := range coinmarketLastData {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					for _, currency := range keys {
						data := coinmarketLastData[currency]
						last, current := data["last"], data["current"]
						text := fmt.Sprintf("Currency: %s \nPrice: %.2f \nDiff: %.2f\n\n", currency, current, current-last)
						buffer.WriteString(text)
					}
					msg := tgbotapi.NewMessage(UID, buffer.String())
					bot.Send(msg)
				}
			case <-doneTicker:
				return
			}
		}
	}()

	for update := range updates {
		if update.Message == nil {
			continue
		}
		var msg tgbotapi.Chattable
		if !update.Message.IsCommand() {
			msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Bot accept only commands.")
			bot.Send(msg)
			continue
		}
		if update.Message.Command() == "subscribe" {
			if _, ok := currentUsers[update.Message.Chat.ID]; ok {
				msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Already subscribed.")
			} else {
				currentUsers[update.Message.Chat.ID] = true
				msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Successfully subscribed.")
			}
			bot.Send(msg)
		} else if update.Message.Command() == "unsubscribe" {
			if _, ok := currentUsers[update.Message.Chat.ID]; !ok {
				msg = tgbotapi.NewMessage(update.Message.Chat.ID, "User not subscribed.")
			} else {
				delete(currentUsers, update.Message.Chat.ID)
				msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Successfully unsubscribed.")
			}
			bot.Send(msg)
		}

		// log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

		// log.Println(update.Message.Chat.ID)

		// msg := tgbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)
		// // msg.ReplyToMessageID = update.Message.MessageID

		// bot.Send(msg)
	}
}
