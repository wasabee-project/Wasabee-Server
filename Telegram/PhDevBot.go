package Telegram

import (
	"bytes"
	// "encoding/json"
	"errors"
	"github.com/cloudkucooland/PhDevBin"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

type TGConfiguration struct {
	APIKey       string
	FrontendPath string
	templateSet  map[string]*template.Template
	teamKbd      tgbotapi.ReplyKeyboardMarkup
	baseKbd      tgbotapi.ReplyKeyboardMarkup
}

var bot *tgbotapi.BotAPI
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
	_ = PhDevBin.PhDevMessagingRegister("Telegram", SendMessage)

	var err error
	bot, err = tgbotapi.NewBotAPI(config.APIKey)
	if err != nil {
		PhDevBin.Log.Error(err)
		return err
	}

	bot.Debug = false
	PhDevBin.Log.Noticef("Authorized to Telegram on account %s", bot.Self.UserName)
	PhDevBin.TGSetBot(&bot.Self)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, _ := bot.GetUpdatesChan(u)
	for update := range updates {
		if update.Message == nil {
			continue
		}

		// PhDevBin.Log.Debugf("[%s] %s", update.Message.From.UserName, update.Message.Text)
		// s, _ := json.MarshalIndent(update.Message, "", "  ")
		// PhDevBin.Log.Debug(string(s))

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
		// PhDevBin.Log.Debug("Language: ", update.Message.From.LanguageCode)
		defaultReply, err := phdevBotTemplateExecute("default", update.Message.From.LanguageCode, nil)
		if err != nil {
			PhDevBin.Log.Error(err)
			continue
		}
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
		tmp, _ := phdevBotTemplateExecute("InitOneFail", inMsg.Message.From.LanguageCode, nil)
		msg.Text = tmp
	} else {
		tmp, _ := phdevBotTemplateExecute("InitOneSuccess", inMsg.Message.From.LanguageCode, nil)
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
		tmp, _ := phdevBotTemplateExecute("InitTwoFail", inMsg.Message.From.LanguageCode, nil)
		msg.Text = tmp
	} else {
		tmp, _ := phdevBotTemplateExecute("InitTwoSuccess", inMsg.Message.From.LanguageCode, nil)
		msg.Text = tmp
	}
	return err
}

func phdevBotTemplates(t map[string]*template.Template) error {
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

	PhDevBin.Log.Debugf("Building Template function map")
	funcMap := template.FuncMap{
		"TGGetBotName": PhDevBin.TGGetBotName,
		"TGGetBotID":   PhDevBin.TGGetBotID,
		"TGRunning":    PhDevBin.TGRunning,
		"Webroot":      PhDevBin.GetWebroot,
		"WebAPIPath":   PhDevBin.GetWebAPIPath,
		"VEnlOne":      PhDevBin.GetvEnlOne,
	}
	config.templateSet = make(map[string]*template.Template)

	if err != nil {
		PhDevBin.Log.Error(err)
	}
	PhDevBin.Log.Notice("Including frontend telegram templates from: ", config.FrontendPath)
	files, err := ioutil.ReadDir(config.FrontendPath)
	if err != nil {
		PhDevBin.Log.Error(err)
	}

	for _, f := range files {
		lang := f.Name()
		if f.IsDir() && len(lang) == 2 {
			config.templateSet[lang] = template.New("").Funcs(funcMap) // one funcMap for all languages
			// load the masters
			config.templateSet[lang].ParseGlob(config.FrontendPath + "/master/*.tg")
			// overwrite with language specific
			config.templateSet[lang].ParseGlob(config.FrontendPath + "/" + lang + "/*.tg")
			PhDevBin.Log.Debugf("Templates for lang [%s] %s", lang, config.templateSet[lang].DefinedTemplates())
		}
	}

	return nil
}

func phdevBotTemplateExecute(name, lang string, data interface{}) (string, error) {
	if lang == "" {
		lang = "en"
	}

	_, ok := config.templateSet[lang]
	if ok == false {
		lang = "en" // default to english if the map doesn't exist
	}

	var tpBuffer bytes.Buffer
	if err := config.templateSet[lang].ExecuteTemplate(&tpBuffer, name, data); err != nil {
		PhDevBin.Log.Notice(err)
		// XXX if lang != "en" { (?retry in en? - the template may be broken) }
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
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Teammates Near Me"),
			tgbotapi.NewKeyboardButton("Farms Near Me"),
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
		switch inMsg.Message.Command() {
		case "start":
			tmp, _ := phdevBotTemplateExecute("help", inMsg.Message.From.LanguageCode, nil)
			msg.Text = tmp
			msg.ReplyMarkup = config.baseKbd
		case "help":
			tmp, _ := phdevBotTemplateExecute("help", inMsg.Message.From.LanguageCode, nil)
			msg.Text = tmp
			msg.ReplyMarkup = config.baseKbd
		default:
			tmp, _ := phdevBotTemplateExecute("default", inMsg.Message.From.LanguageCode, nil)
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
		switch cmd {
		case "Home":
			msg.ReplyMarkup = config.baseKbd
			msg.Text = "Home"
		case "Teams":
			tmp, _ := phdevBotTeamKeyboard(gid)
			msg.ReplyMarkup = tmp
			msg.Text = "Teams"
		case "On:":
			msg.Text, _ = phdevBotTemplateExecute("TeamStateChange", inMsg.Message.From.LanguageCode, tStruct{
				State: "On",
				Team:  name,
			})
			PhDevBin.SetUserTeamStateName(gid, name, "On")
			msg.ReplyMarkup, _ = phdevBotTeamKeyboard(gid)
		case "Off:":
			msg.Text, _ = phdevBotTemplateExecute("TeamStateChange", inMsg.Message.From.LanguageCode, tStruct{
				State: "Off",
				Team:  name,
			})
			PhDevBin.SetUserTeamStateName(gid, name, "Off")
			msg.ReplyMarkup, _ = phdevBotTeamKeyboard(gid)
		case "Primary:":
			msg.Text, _ = phdevBotTemplateExecute("TeamStateChange", inMsg.Message.From.LanguageCode, tStruct{
				State: "Primary",
				Team:  name,
			})
			PhDevBin.SetUserTeamStateName(gid, name, "Primary")
			msg.ReplyMarkup, _ = phdevBotTeamKeyboard(gid)
		case "Teammates":
			msg.Text, _ = teammatesNear(gid, inMsg)
			msg.ReplyMarkup = config.baseKbd
			msg.DisableWebPagePreview = true
		case "Farms":
			msg.Text, _ = farmsNear(gid, inMsg)
			msg.ReplyMarkup = config.baseKbd
			msg.DisableWebPagePreview = true
		case "Targets":
			msg.Text, _ = targetsNear(gid, inMsg)
			msg.ReplyMarkup = config.baseKbd
			msg.DisableWebPagePreview = true
		default:
			msg.ReplyMarkup = config.baseKbd
		}
	}

	if inMsg.Message.Location != nil {
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

func SendMessage(gid, message string) (bool, error) {
	tgid, err := PhDevBin.GidToTelegram(gid)
	if err != nil {
		PhDevBin.Log.Notice(err)
		return false, err
	}
	if tgid == 0 {
		err = errors.New("Telegram ID not found")
		PhDevBin.Log.Notice(err)
		return false, err
	}
	msg := tgbotapi.NewMessage(tgid, "")
	msg.Text = message
	msg.ParseMode = "MarkDown"

	bot.Send(msg)
	PhDevBin.Log.Notice("Sent message to:", gid)
	return true, nil
}

func teammatesNear(gid string, inMsg *tgbotapi.Update) (string, error) {
	var td PhDevBin.TeamData
	var txt = ""
	maxdistance := 500
	maxresults := 10

	err := PhDevBin.TeammatesNearGid(gid, maxdistance, maxresults, &td)
	if err != nil {
		PhDevBin.Log.Error(err)
		return txt, err
	}
	txt, err = phdevBotTemplateExecute("Teammates", inMsg.Message.From.LanguageCode, &td)

	return txt, err
}

func targetsNear(gid string, inMsg *tgbotapi.Update) (string, error) {
	var td PhDevBin.TeamData
	var txt = ""
	maxdistance := 100
	maxresults := 10

	err := PhDevBin.TargetsNearGid(gid, maxdistance, maxresults, &td)
	if err != nil {
		PhDevBin.Log.Error(err)
		return txt, err
	}
	txt, err = phdevBotTemplateExecute("Targets", inMsg.Message.From.LanguageCode, &td)

	return txt, err
}

func farmsNear(gid string, inMsg *tgbotapi.Update) (string, error) {
	var td PhDevBin.TeamData
	var txt = ""
	maxdistance := 100
	maxresults := 10

	err := PhDevBin.TargetsNearGid(gid, maxdistance, maxresults, &td)
	if err != nil {
		PhDevBin.Log.Error(err)
		return txt, err
	}
	txt, err = phdevBotTemplateExecute("Farms", inMsg.Message.From.LanguageCode, &td)

	return txt, err
}
