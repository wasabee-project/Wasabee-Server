package wtg

import (
	"fmt"
	"html/template"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/generatename"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/templates"
)

// Config is the main configuration data for the Telegram interface
// passed to main() pre-loaded with APIKey and TemplateSet set, the rest is built when the bot starts
type Config struct {
	APIKey      string
	HookPath    string
	TemplateSet map[string]*template.Template
	baseKbd     tgbotapi.ReplyKeyboardMarkup
	upChan      chan tgbotapi.Update
	hook        string
}

var bot *tgbotapi.BotAPI
var c Config

// WasabeeBot is called from main() to start the bot.
func WasabeeBot(in *Config) {
	if in.APIKey == "" {
		log.Infow("startup", "subsystem", "Telegram", "message", "Telegram API key not set; not starting")
		return
	}
	c.APIKey = in.APIKey

	if in.TemplateSet == nil {
		log.Warnw("startup", "subsystem", "Telegram", "message", "the Telegram message templates are not loaded; not starting Telegram bot")
		return
	}
	c.TemplateSet = in.TemplateSet
	c.baseKbd = keyboards()
	c.HookPath = in.HookPath
	if c.HookPath == "" {
		c.HookPath = "/tg"
	}

	c.upChan = make(chan tgbotapi.Update, 10) // not using bot.ListenForWebhook() since we need our own bidirectional channel
	webhook := config.Subrouter(c.HookPath)
	webhook.HandleFunc("/{hook}", TGWebHook).Methods("POST")

	var err error
	bot, err = tgbotapi.NewBotAPI(c.APIKey)
	if err != nil {
		log.Error(err)
		return
	}

	// bot.Debug = true
	log.Infow("startup", "subsystem", "Telegram", "message", "authorized to Telegram as "+bot.Self.UserName)
	config.TGSetBot(bot.Self.UserName, int(bot.Self.ID))

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	webroot := config.GetWebroot()
	c.hook = generatename.GenerateName()
	whurl := fmt.Sprintf("%s%s/%s", webroot, c.HookPath, c.hook)
	wh, _ := tgbotapi.NewWebhook(whurl)
	if _, err = bot.Request(wh); err != nil {
		log.Error(err)
		return
	}

	// let the messaging susbsystem know we exist and how to use us
	messaging.RegisterMessageBus("Telegram", messaging.Bus{
		SendMessage:      sendMessage,
		SendTarget:       sendTarget,
		AddToRemote:      addToChat,
		RemoveFromRemote: removeFromChat,
	})

	i := 1
	for update := range c.upChan {
		// log.Debugf("running update: %s", update)
		if err = runUpdate(update); err != nil {
			log.Error(err)
			continue
		}
		if (i % 100) == 0 { // every 100 requests, change the endpoint
			i = 1
			c.hook = generatename.GenerateName()
			whurl = fmt.Sprintf("%s%s/%s", webroot, c.HookPath, c.hook)
			wh, _ := tgbotapi.NewWebhook(whurl)
			_, err = bot.Request(wh)
			if err != nil {
				log.Error(err)
			}
		}
		i++
	}
}

// Shutdown closes all the Telegram connections
func Shutdown() {
	log.Infow("shutdown", "subsystem", "Telegram", "message", "shutdown telegram")
	bot.StopReceivingUpdates()
}

func runUpdate(update tgbotapi.Update) error {
	if update.CallbackQuery != nil {
		log.Debugw("callback", "subsystem", "Telegram", "data", update)
		msg, err := callback(&update)
		if err != nil {
			log.Error(err)
			return err
		}
		if _, err = bot.Send(msg); err != nil {
			log.Error(err)
			return err
		}
		/* if _, err = bot.DeleteMessage(tgbotapi.NewDeleteMessage(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID)); err != nil {
			log.Error(err)
			return err
		} */
		return nil
	}

	if update.Message != nil {
		if update.Message.Chat.Type == "private" {
			if err := processDirectMessage(&update); err != nil {
				log.Error(err)
			}
		} else {
			if err := processChatMessage(&update); err != nil {
				log.Error(err)
			}
		}
	}

	if update.EditedMessage != nil && update.EditedMessage.Location != nil {
		if err := liveLocationUpdate(&update); err != nil {
			log.Error(err)
		}
	}

	return nil
}

func keyboards() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButtonLocation("Send Location"),
			tgbotapi.NewKeyboardButton("Tasks"),
		),
	)
}

// sendMessage is registered with Wasabee-Server as a message bus to allow other modules to send messages via Telegram
func sendMessage(g messaging.GoogleID, message string) (bool, error) {
	gid := model.GoogleID(g)
	tgid, err := gid.TelegramID()
	if err != nil {
		log.Error(err)
		return false, err
	}
	tgid64 := int64(tgid)
	if tgid64 == 0 {
		log.Debugw("TelegramID not found", "subsystem", "Telegram", "GID", gid)
		return false, nil
	}
	msg := tgbotapi.NewMessage(tgid64, "")
	msg.Text = message
	msg.ParseMode = "HTML"

	_, err = bot.Send(msg)
	if err != nil && err.Error() != "Bad Request: chat not found" {
		log.Error(err)
		return false, err
	}
	if err != nil && err.Error() == "Bad Request: chat not found" {
		log.Debugw(err.Error(), "gid", gid, "tgid", tgid)
		return false, nil
	}

	log.Debugw("sent message", "subsystem", "Telegram", "GID", gid)
	return true, nil
}

// sendTarget is used to send a formatted target to an agent
func sendTarget(g messaging.GoogleID, target messaging.Target) error {
	gid := model.GoogleID(g)
	tgid, err := gid.TelegramID()
	if err != nil {
		log.Error(err)
		return err
	}
	tgid64 := int64(tgid)
	if tgid64 == 0 {
		log.Debugw("TelegramID not found", "subsystem", "Telegram", "GID", gid)
		return nil
	}
	msg := tgbotapi.NewMessage(tgid64, "")
	msg.ParseMode = "HTML"

	// Lng vs Lon ...
	templateData := struct {
		Name   string
		ID     string
		Lat    string
		Lon    string
		Type   string
		Sender string
	}{
		Name:   target.Name,
		ID:     target.ID,
		Lat:    target.Lat,
		Lon:    target.Lng,
		Type:   target.Type,
		Sender: target.Name,
	}

	msg.Text, err = templates.Execute("target", templateData)
	if err != nil {
		log.Error(err)
		msg.Text = fmt.Sprintf("template failed; target @ %s %s", target.Lat, target.Lng)
	}

	_, err = bot.Send(msg)
	if err != nil && err.Error() != "Bad Request: chat not found" {
		log.Error(err)
		return err
	}
	if err != nil && err.Error() == "Bad Request: chat not found" {
		log.Debugw(err.Error(), "gid", gid, "tgid", tgid)
		return err
	}

	log.Debugw("sent target", "subsystem", "Telegram", "GID", gid, "target", target)
	return nil
}
