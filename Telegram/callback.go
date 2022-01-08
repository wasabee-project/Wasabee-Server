package wtg

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

// callback is where to determine which callback is called, and what to do with it
func callback(update *tgbotapi.Update) (tgbotapi.MessageConfig, error) {
	log.Debug("callback", "query", update.CallbackQuery)

	msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
	gid, err := model.TelegramID(update.CallbackQuery.From.ID).Gid()
	if err != nil {
		log.Error(err)
		return msg, err
	}

	if update.CallbackQuery.Message.Location != nil && update.CallbackQuery.Message.Location.Latitude != 0 {
		log.Debug("location in callback?")
		lat := strconv.FormatFloat(update.CallbackQuery.Message.Location.Latitude, 'f', -1, 64)
		lon := strconv.FormatFloat(update.CallbackQuery.Message.Location.Longitude, 'f', -1, 64)
		if err = gid.SetLocation(lat, lon); err != nil {
			log.Error(err)
			return msg, err
		}
		msg.Text = "Location Processed"
	}

	if update.CallbackQuery.Message.Chat.Type != "private" {
		log.Error("callbacks valid only in private chat: " + update.CallbackQuery.Message.Chat.Type)
		return msg, nil
	}

	// can we switch this to JSON?
	// data is in format class/action/id e.g. "team/deactivate/wibbly-wobbly-9988"
	command := strings.SplitN(update.CallbackQuery.Data, "/", 3)
	if len(command) == 0 {
		err := fmt.Errorf("callback wthout command")
		log.Error(err)
		return msg, err
	}
	switch command[0] {
	// add other commands here
	case "wasabee":
		msg.Text = "wasabee rocks"
	default:
		resp, err := bot.Request(
			tgbotapi.CallbackConfig{CallbackQueryID: update.CallbackQuery.ID, Text: "Unknown Callback"},
		)
		if err != nil {
			log.Error(err)
			return msg, err
		}
		if !resp.Ok {
			log.Error(resp.Description)
		}
	}
	return msg, nil
}
