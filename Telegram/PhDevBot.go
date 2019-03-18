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
	// make this a []*template.Template, one for each language...
	templateSet *template.Template
	teamKbd     tgbotapi.ReplyKeyboardMarkup
	baseKbd     tgbotapi.ReplyKeyboardMarkup
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
		msg.ParseMode = "MarkDown"
		// s, _ := json.MarshalIndent(msg, "", "  ")
		// PhDevBin.Log.Debug(string(s))

		gid, verified, err := PhDevBin.TelegramToGid(update.Message.From.ID)
		if err != nil {
			PhDevBin.Log.Error(err)
			continue
		}

		if gid == "" {
			PhDevBin.Log.Debugf("Unknown user: %s (%s); initializing", update.Message.From.UserName, string(update.Message.From.ID))
			err = phdevBotNewUser_Init(&msg, &update)
			if err != nil {
				PhDevBin.Log.Error(err)
			}
		} else if verified == false {
			PhDevBin.Log.Debugf("Unverified user: %s (%s); verifying", update.Message.From.UserName, string(update.Message.From.ID))
			err = phdevBotNewUser_Verify(&msg, &update)
			if err != nil {
				PhDevBin.Log.Error(err)
			}
		} else { // verified user, process message
			if err = phdevBotMessage(&msg, &update, gid); err != nil {
				PhDevBin.Log.Error(err)
			}
		}
		if msg.Text != "" {
			msg.ReplyToMessageID = update.Message.MessageID
		}

		bot.Send(msg)
	}

	return nil
}

func phdevBotNewUser_Init(msg *tgbotapi.MessageConfig, inMsg *tgbotapi.Update) error {
	var lockey string
	if inMsg.Message.IsCommand() {
		tokens := strings.Split(inMsg.Message.Text, " ")
		if len(tokens) == 2 {
			lockey = tokens[1]
		}
	} else {
		lockey = inMsg.Message.Text
	}
	strings.TrimSpace(lockey)
	err := PhDevBin.TelegramInitUser(inMsg.Message.From.ID, inMsg.Message.From.UserName, lockey)
	if err != nil {
		PhDevBin.Log.Error(err)
		tmp, _ := phdevBotTemplateExecute("InitOneFail", nil)
		msg.Text = tmp
	} else {
		tmp, _ := phdevBotTemplateExecute("InitOneSuccess", nil)
		msg.Text = tmp
	}
	return err
}

func phdevBotNewUser_Verify(msg *tgbotapi.MessageConfig, inMsg *tgbotapi.Update) error {
	var authtoken string
	if inMsg.Message.IsCommand() {
		tokens := strings.Split(inMsg.Message.Text, " ")
		if len(tokens) == 2 {
			authtoken = tokens[1]
		}
	} else {
		authtoken = inMsg.Message.Text
	}
	strings.TrimSpace(authtoken)
	err := PhDevBin.TelegramInitUser2(inMsg.Message.From.ID, authtoken)
	if err != nil {
		PhDevBin.Log.Error(err)
		tmp, _ := phdevBotTemplateExecute("InitTwoFail", nil)
		msg.Text = tmp
	} else {
		tmp, _ := phdevBotTemplateExecute("InitTwoSuccess", nil)
		msg.Text = tmp
	}
	return err
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
		"WebAPIPath":   PhDevBin.GetWebAPIPath,
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
			tgbotapi.NewKeyboardButton("Targets Near Me"),
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
	var i int

	for _, v := range ud.Teams {
		i++
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

		if i > 5 { // too many rows and the screen fills up
			break
		}
	}

	home := tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton("Home"))
	rows = append(rows, home)

	// api isn't dynamic-list friendly, roll my own data
	tmp := tgbotapi.ReplyKeyboardMarkup{
		Keyboard:       rows,
		ResizeKeyboard: true,
	}
	return tmp, nil

}

// This is where command processing takes place
func phdevBotMessage(msg *tgbotapi.MessageConfig, inMsg *tgbotapi.Update, gid string) error {
	if inMsg.Message.IsCommand() {
		PhDevBin.Log.Debug("Found command", inMsg.Message.Command())
		switch inMsg.Message.Command() {
		case "start":
			tmp, _ := phdevBotTemplateExecute("help", nil)
			msg.Text = tmp
			msg.ReplyMarkup = config.baseKbd
		case "help":
			tmp, _ := phdevBotTemplateExecute("help", nil)
			msg.Text = tmp
			msg.ReplyMarkup = config.baseKbd
		default:
			tmp, _ := phdevBotTemplateExecute("default", nil)
			msg.Text = tmp
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
		case "Targets":
			msg.Text = "No Nearby Targets Found"
			msg.ReplyMarkup = config.baseKbd
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
		msg.Text = "Location Processed"
	}
	return nil
}
