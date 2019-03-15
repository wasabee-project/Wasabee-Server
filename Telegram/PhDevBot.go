package PhDevTelegram

import (
	"bytes"
	"errors"
	"github.com/cloudkucooland/PhDevBin"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"path/filepath"
	"strings"
	"text/template"
)

type TGConfiguration struct {
	APIKey       string
	FrontendPath string
	templateSet  *template.Template
}

var config TGConfiguration

func PhDevBot(conf TGConfiguration) error {
	if conf.APIKey == "" {
		err := errors.New("API Key not set")
		PhDevBin.Log.Critical(err)
		return err
	}

	if config.FrontendPath == "" {
		config.FrontendPath = "frontend"
	}
	_ = phdevBotTemplates(config.templateSet)

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

	defaultReply, err := phdevBotTemplateExecute("default", nil)
	if err != nil {
		PhDevBin.Log.Critical(err)
		return (err)
	}

	updates, err := bot.GetUpdatesChan(u)
	for update := range updates {
		if update.Message == nil {
			continue
		}

		PhDevBin.Log.Noticef("[%s] %s", update.Message.From.UserName, update.Message.Text)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
		msg.Text = defaultReply

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
				PhDevBin.Log.Debug("Found command", update.Message.Command())
				switch update.Message.Command() {
				case "help":
					tmp, _ := phdevBotTemplateExecute("help", nil)
					msg.Text = tmp
				case "teams":
					var ud PhDevBin.UserData
					err = PhDevBin.GetUserData(gid, &ud)
					if err != nil {
						PhDevBin.Log.Notice(err)
						continue
					}
					tmp, err := phdevBotTemplateExecute("teams", &ud)
					if err != nil {
						PhDevBin.Log.Notice(err)
						continue
					}
					msg.Text = tmp
				default:
					msg.Text = "Unrecognized Command"
				}
			}
			/* if update.Message.IsLocation() {
				PhDevBin.Log.Debug("got location")
			} */
		}

		msg.ReplyToMessageID = update.Message.MessageID
		bot.Send(msg)
	}

	return nil
}

// XXX move constants to templates
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

func phdevBotTemplates(t *template.Template) error {
	if config.FrontendPath == "" {
		err := errors.New("FrontendPath not configured")
		PhDevBin.Log.Critical(err)
		return err
	}

	frontendPath, err := filepath.Abs(config.FrontendPath)
	if err != nil {
		PhDevBin.Log.Critical("Frontend path couldn't be resolved.")
		panic(err)
	}
	config.FrontendPath = frontendPath

	PhDevBin.Log.Debugf("Loading Template function map")
	funcMap := template.FuncMap{
		"TGGetBotName": PhDevBin.TGGetBotName,
		"TGGetBotID":   PhDevBin.TGGetBotID,
		"TGRunning":    PhDevBin.TGRunning,
	}
	config.templateSet = template.New("").Funcs(funcMap)
	if err != nil {
		PhDevBin.Log.Error(err)
	}
	PhDevBin.Log.Notice("Including frontend telegram templates from: ", config.FrontendPath)
	config.templateSet.ParseGlob(config.FrontendPath + "/*.tg")
	PhDevBin.Log.Debug(config.templateSet.DefinedTemplates())

	return nil
}

func phdevBotTemplateExecute(name string, data interface{}) (string, error) {
	var tpBuffer bytes.Buffer
	if err := config.templateSet.ExecuteTemplate(&tpBuffer, name, data); err != nil {
		PhDevBin.Log.Notice(err)
		return "", err
	}
	return tpBuffer.String(), nil
}
