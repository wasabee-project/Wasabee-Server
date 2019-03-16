package PhDevTelegram

import (
	"bytes"
	// "encoding/json"
	"errors"
	"github.com/cloudkucooland/PhDevBin"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

type TGConfiguration struct {
	APIKey       string
	FrontendPath string
	templateSet  *template.Template
	teamKbd      tgbotapi.ReplyKeyboardMarkup
	baseKbd      tgbotapi.ReplyKeyboardMarkup
}

var config TGConfiguration

func PhDevBot(init TGConfiguration) error {
	if init.APIKey == "" {
		err := errors.New("API Key not set")
		PhDevBin.Log.Critical(err)
		return err
	}
	config.APIKey = init.APIKey

	config.FrontendPath = init.FrontendPath
	if config.FrontendPath == "" {
		config.FrontendPath = "frontend"
	}
	_ = phdevBotTemplates(config.templateSet)
	_ = phdevBotKeyboards(&config)

	bot, err := tgbotapi.NewBotAPI(config.APIKey)
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

		PhDevBin.Log.Debugf("[%s] %s", update.Message.From.UserName, update.Message.Text)
		// s, _ := json.MarshalIndent(update.Message, "", "  ")
		// PhDevBin.Log.Debug(string(s))

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
				continue
			}
		} else {
			if err = phdevBotMessage(&msg, &update, gid); err != nil {
				PhDevBin.Log.Error(err)
				continue
			}
		}
		if msg.Text != "" {
			msg.ReplyToMessageID = update.Message.MessageID
		}

		bot.Send(msg)
	}

	return nil
}

// XXX move constants to templates
func phdevBotNewUser(msg *tgbotapi.MessageConfig, inMsg *tgbotapi.Update) error {
	if inMsg.Message.IsCommand() {
		switch inMsg.Message.Command() {
		case "id":
			tokens := strings.Split(inMsg.Message.Text, " ")
			if len(tokens) != 2 || tokens[1] == "" {
				msg.Text = "See instructions at https://qbin.phtiv.com:443/me to get started"
				return errors.New("/id {location share key} missing or too many words")
			}
			lockey := tokens[1]
			err := PhDevBin.TelegramInitUser(inMsg.Message.From.ID, inMsg.Message.From.UserName, lockey)
			if err != nil {
				PhDevBin.Log.Error(err)
				msg.Text = err.Error()
				return err
			} else {
				msg.Text = "Go to https://qbin.phtiv.com:8443/me and get your telegram authkey and send it to me with the /setkey {telegram-key} command"
			}
		case "setkey":
			tokens := strings.Split(inMsg.Message.Text, " ")
			if len(tokens) != 2 || tokens[1] == "" {
				msg.Text = "Please send your verification key: /setkey {verification key}"
				return errors.New("/setkey {verification key} missing or too many words")
			}
			key := tokens[1]
			err := PhDevBin.TelegramInitUser2(inMsg.Message.From.ID, key)
			if err != nil {
				PhDevBin.Log.Error(err)
				msg.Text = err.Error()
				return err
			} else {
				msg.Text = "Verified."
			}
		default:
			msg.Text = "See instructions at https://qbin.phtiv.com:8443/me to get started"
		}
	} else {
		msg.Text = "See instructions at https://qbin.phtiv.com:8443/me to get started"
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
		"Webroot":      PhDevBin.GetWebroot,
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

func phdevBotKeyboards(c *TGConfiguration) error {
	c.teamKbd = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Home"),
		),
	)

	c.baseKbd = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButtonLocation("Send Location"),
			tgbotapi.NewKeyboardButton("Teams"),
		),
	)

	return nil
}

func phdevBotTeamKeyboard(gid string) (tgbotapi.ReplyKeyboardMarkup, error) {
	var ud PhDevBin.UserData
	if err := PhDevBin.GetUserData(gid, &ud); err != nil {
		return config.teamKbd, err
	}

	var rows [][]tgbotapi.KeyboardButton

	for _, v := range ud.Teams {
		var on, off, primary tgbotapi.KeyboardButton
		if v.State != "On" {
			on = tgbotapi.NewKeyboardButton("On: " + v.Name)
		}
		if v.State != "Off" {
			off = tgbotapi.NewKeyboardButton("Off: " + v.Name)
		}
		if v.State != "Primary" {
			primary = tgbotapi.NewKeyboardButton("Primary: " + v.Name)
		}
		q := tgbotapi.NewKeyboardButtonRow(on, off, primary)
		rows = append(rows, q)
	}

	home := tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton("Home"))
	rows = append(rows, home)

	// api isn't dynamic friendly
	tmp := tgbotapi.ReplyKeyboardMarkup{
		Keyboard:       rows,
		ResizeKeyboard: true,
	}
	return tmp, nil

}

func phdevBotMessage(msg *tgbotapi.MessageConfig, inMsg *tgbotapi.Update, gid string) error {
	if inMsg.Message.IsCommand() {
		PhDevBin.Log.Debug("Found command", inMsg.Message.Command())
		switch inMsg.Message.Command() {
		case "help":
			tmp, _ := phdevBotTemplateExecute("help", nil)
			msg.Text = tmp
			msg.ReplyMarkup = config.baseKbd
		case "teams":
			var ud PhDevBin.UserData
			err := PhDevBin.GetUserData(gid, &ud)
			if err != nil {
				PhDevBin.Log.Notice(err)
				return err
			}
			tmp, err := phdevBotTemplateExecute("teams", &ud)
			if err != nil {
				PhDevBin.Log.Notice(err)
				return err
			}
			msg.Text = tmp
			msg.ReplyMarkup, err = phdevBotTeamKeyboard(gid)
		default:
			msg.Text = "Unrecognized Command"
			msg.ReplyMarkup = config.baseKbd
		}
	} else if inMsg.Message.Text != "" {
		type tStruct struct {
			State string
			Team  string
		}
		// get first word
		tokens := strings.Split(inMsg.Message.Text, " ")
		cmd := tokens[0]

		// get the rest
		var name string
		x := len(cmd)
		if x+1 < len(inMsg.Message.Text) {
			name = inMsg.Message.Text[x+1:]
		}
		PhDevBin.Log.Debugf("Command [%s]", cmd)
		PhDevBin.Log.Debugf("name [%s]", name)
		switch cmd {
		case "Home":
			msg.ReplyMarkup = config.baseKbd
			msg.Text = "Home"
		case "Teams":
			tmp, _ := phdevBotTeamKeyboard(gid)
			msg.ReplyMarkup = tmp
			msg.Text = "Teams"
		case "On:":
			msg.Text, _ = phdevBotTemplateExecute("TeamStateChange", tStruct{
				State: "On",
				Team:  name,
			})
			PhDevBin.SetUserTeamStateName(gid, name, "On")
			msg.ReplyMarkup, _ = phdevBotTeamKeyboard(gid)
		case "Off:":
			msg.Text, _ = phdevBotTemplateExecute("TeamStateChange", tStruct{
				State: "Off",
				Team:  name,
			})
			PhDevBin.SetUserTeamStateName(gid, name, "Off")
			msg.ReplyMarkup, _ = phdevBotTeamKeyboard(gid)
		case "Primary:":
			msg.Text, _ = phdevBotTemplateExecute("TeamStateChange", tStruct{
				State: "Primary",
				Team:  name,
			})
			PhDevBin.SetUserTeamStateName(gid, name, "Primary")
			msg.ReplyMarkup, _ = phdevBotTeamKeyboard(gid)
		default:
			msg.ReplyMarkup = config.baseKbd
		}
	}

	if inMsg.Message.Location != nil {
		PhDevBin.Log.Debug("Got TG Location")
		PhDevBin.UserLocation(gid,
			strconv.FormatFloat(inMsg.Message.Location.Latitude, 'f', -1, 64),
			strconv.FormatFloat(inMsg.Message.Location.Longitude, 'f', -1, 64),
			"Telegram",
		)
		msg.ReplyMarkup = config.baseKbd
		msg.Text = ""
	}
	return nil
}
