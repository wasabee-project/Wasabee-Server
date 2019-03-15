package PhDevTelegram

import (
	"errors"
	"github.com/cloudkucooland/PhDevBin"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"strings"
)

type Configuration struct {
	APIKey string
}

var config Configuration

func PhDevBot(conf Configuration) error {
	if conf.APIKey == "" {
		return errors.New("API Key not set")
	}

	bot, err := tgbotapi.NewBotAPI(conf.APIKey)
	if err != nil {
		PhDevBin.Log.Error(err)
		return err
	}

	bot.Debug = false
	PhDevBin.Log.Noticef("Authorized to Telegram on account %s", bot.Self.UserName)
	PhDevBin.TGSetBot(&bot.Self)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		PhDevBin.Log.Noticef("[%s] %s", update.Message.From.UserName, update.Message.Text)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")

		gid, verified, err := PhDevBin.TelegramToGid(update.Message.From.ID)
		if err != nil {
			PhDevBin.Log.Error(err)
			continue
		}

		if gid == "" || verified == false {
			PhDevBin.Log.Debugf("Unknown user: %s (%s); initializing", update.Message.From.UserName, string(update.Message.From.ID))
			if err = phdevBotNewUser(&msg, &update); err != nil {
				PhDevBin.Log.Error(err)
			}
		} else {
			if update.Message.IsCommand() {
				switch update.Message.Command() {
				case "help":
					msg.Text = "coming soon"
				case "teams":
					msg.Text = "list of teams"
				default:
					msg.Text = "Unrecognized Command"
				}
			} else {
				msg.Text = "I don't understand"
			}
		}

		msg.ReplyToMessageID = update.Message.MessageID
		bot.Send(msg)
	}

	return nil
}

func phdevBotNewUser(msg *tgbotapi.MessageConfig, update *tgbotapi.Update) error {
	if update.Message.IsCommand() {
		switch update.Message.Command() {
		case "id":
			tokens := strings.Split(update.Message.Text, " ")
			if len(tokens) != 2 || tokens[1] == "" {
				msg.Text = "You must set a Location Share Key: /id {location share key}"
				return errors.New("/id {location share key} missing or too many words")
			}
			lockey := tokens[1]
			err := PhDevBin.TelegramInitUser(update.Message.From.ID, update.Message.From.UserName, lockey)
			if err != nil {
				PhDevBin.Log.Error(err)
				msg.Text = err.Error()
				return err
			} else {
				msg.Text = "Go to http://qbin.phtiv.com:8443/me and get your telegram authkey and send it to me with the /setkey {telegram-key} command"
			}
		case "setkey":
			tokens := strings.Split(update.Message.Text, " ")
			if len(tokens) != 2 || tokens[1] == "" {
				msg.Text = "Please send your verification key: /setkey {verification key}"
				return errors.New("/setkey {verification key} missing or too many words")
			}
			key := tokens[1]
			err := PhDevBin.TelegramInitUser2(update.Message.From.ID, key)
			if err != nil {
				PhDevBin.Log.Error(err)
				msg.Text = err.Error()
				return err
			} else {
				msg.Text = "Verified."
			}
		default:
			msg.Text = "Use /id {location share key}"
		}
	} else {
		msg.Text = "Please send your location share key with /id {location share key}"
	}
	return nil
}
