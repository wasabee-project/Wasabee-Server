package PhDevTelegram

import (
	"errors"
	"github.com/cloudkucooland/PhDevBin"
	"github.com/go-telegram-bot-api/telegram-bot-api"
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

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		PhDevBin.Log.Noticef("[%s] %s", update.Message.From.UserName, update.Message.Text)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")

		gid, err := PhDevBin.TelegramToGid(update.Message.From.UserName)
		if err != nil {
			PhDevBin.Log.Error(err)
			continue
		}
		if gid == "" {
			PhDevBin.Log.Errorf("Unknown user: %s; initializing", update.Message.From.UserName)
			if msg.Text, err = phdevBotNewUser(update.Message.From.UserName); err != nil {
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

func phdevBotNewUser(tgname string) (string, error) {
	// start setting them up
	return "Still working on it", nil
}
