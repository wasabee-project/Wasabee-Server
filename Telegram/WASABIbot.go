package wasabitelegram

import (
	"bytes"
	// "encoding/json"
	"errors"
	"fmt"
	"github.com/cloudkucooland/WASABI"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

// TGConfiguration is the main configuration data for the Telegram interface
// passed to main() pre-loaded with APIKey and FrontendPath set, the rest is built when the bot starts
type TGConfiguration struct {
	APIKey       string
	FrontendPath string
	templateSet  map[string]*template.Template
	teamKbd      tgbotapi.ReplyKeyboardMarkup
	baseKbd      tgbotapi.ReplyKeyboardMarkup
}

var bot *tgbotapi.BotAPI
var config TGConfiguration

// WASABIBot is called from main() to start the bot.
func WASABIBot(init TGConfiguration) error {
	if init.APIKey == "" {
		err := errors.New("API Key not set")
		wasabi.Log.Info(err)
		return err
	}
	config.APIKey = init.APIKey

	config.FrontendPath = init.FrontendPath
	if config.FrontendPath == "" {
		config.FrontendPath = "frontend"
	}
	_ = wasabibotTemplates(config.templateSet)
	_ = wasabibotKeyboards(&config)
	_ = wasabi.RegisterMessageBus("Telegram", SendMessage)

	var err error
	bot, err = tgbotapi.NewBotAPI(config.APIKey)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}

	bot.Debug = false
	wasabi.Log.Noticef("Authorized to Telegram on account %s", bot.Self.UserName)
	wasabi.TGSetBot(bot.Self.UserName, bot.Self.ID)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, _ := bot.GetUpdatesChan(u)
	for update := range updates {
		if update.Message == nil {
			continue
		}

		// wasabi.Log.Debugf("[%s] %s", update.Message.From.UserName, update.Message.Text)
		// s, _ := json.MarshalIndent(update.Message, "", "  ")
		// wasabi.Log.Debug(string(s))

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
		// wasabi.Log.Debug("Language: ", update.Message.From.LanguageCode)
		defaultReply, err := wasabibotTemplateExecute("default", update.Message.From.LanguageCode, nil)
		if err != nil {
			wasabi.Log.Error(err)
			continue
		}
		msg.Text = defaultReply
		msg.ParseMode = "MarkDown"
		// s, _ := json.MarshalIndent(msg, "", "  ")
		// wasabi.Log.Debug(string(s))

		tgid := wasabi.TelegramID(update.Message.From.ID)
		gid, verified, err := tgid.GidV()

		if err != nil {
			wasabi.Log.Error(err)
			continue
		}

		if gid == "" {
			wasabi.Log.Debugf("unknown user: %s (%s); initializing", update.Message.From.UserName, string(update.Message.From.ID))
			err = wasabibotNewUserInit(&msg, &update)
			if err != nil {
				wasabi.Log.Error(err)
			}
		} else if !verified {
			wasabi.Log.Debugf("unverified user: %s (%s); verifying", update.Message.From.UserName, string(update.Message.From.ID))
			err = wasabibotNewUserVerify(&msg, &update)
			if err != nil {
				wasabi.Log.Error(err)
			}
		} else { // verified user, process message
			if err = wasabibotMessage(&msg, &update, gid); err != nil {
				wasabi.Log.Error(err)
			}
		}

		bot.Send(msg)
		bot.DeleteMessage(tgbotapi.NewDeleteMessage(update.Message.Chat.ID, update.Message.MessageID))
	}

	return nil
}

func wasabibotNewUserInit(msg *tgbotapi.MessageConfig, inMsg *tgbotapi.Update) error {
	var lockey wasabi.LocKey
	if inMsg.Message.IsCommand() {
		tokens := strings.Split(inMsg.Message.Text, " ")
		if len(tokens) == 2 {
			lockey = wasabi.LocKey(strings.TrimSpace(tokens[1]))
		}
	} else {
		lockey = wasabi.LocKey(strings.TrimSpace(inMsg.Message.Text))
	}

	tid := wasabi.TelegramID(inMsg.Message.From.ID)
	err := tid.TelegramInitAgent(inMsg.Message.From.UserName, lockey)
	if err != nil {
		wasabi.Log.Error(err)
		tmp, _ := wasabibotTemplateExecute("InitOneFail", inMsg.Message.From.LanguageCode, nil)
		msg.Text = tmp
	} else {
		tmp, _ := wasabibotTemplateExecute("InitOneSuccess", inMsg.Message.From.LanguageCode, nil)
		msg.Text = tmp
	}
	return err
}

func wasabibotNewUserVerify(msg *tgbotapi.MessageConfig, inMsg *tgbotapi.Update) error {
	var authtoken string
	if inMsg.Message.IsCommand() {
		tokens := strings.Split(inMsg.Message.Text, " ")
		if len(tokens) == 2 {
			authtoken = tokens[1]
		}
	} else {
		authtoken = inMsg.Message.Text
	}
	authtoken = strings.TrimSpace(authtoken)
	tid := wasabi.TelegramID(inMsg.Message.From.ID)
	err := tid.TelegramVerifyUser(authtoken)
	if err != nil {
		wasabi.Log.Error(err)
		tmp, _ := wasabibotTemplateExecute("InitTwoFail", inMsg.Message.From.LanguageCode, nil)
		msg.Text = tmp
	} else {
		tmp, _ := wasabibotTemplateExecute("InitTwoSuccess", inMsg.Message.From.LanguageCode, nil)
		msg.Text = tmp
	}
	return err
}

func wasabibotTemplates(t map[string]*template.Template) error {
	if config.FrontendPath == "" {
		err := errors.New("FrontendPath not configured")
		wasabi.Log.Critical(err)
		return err
	}

	frontendPath, err := filepath.Abs(config.FrontendPath)
	if err != nil {
		wasabi.Log.Critical("Frontend path couldn't be resolved.")
		panic(err)
	}
	config.FrontendPath = frontendPath

	wasabi.Log.Debugf("Building Template function map")
	funcMap := template.FuncMap{
		"TGGetBotName": wasabi.TGGetBotName,
		"TGGetBotID":   wasabi.TGGetBotID,
		"TGRunning":    wasabi.TGRunning,
		"Webroot":      wasabi.GetWebroot,
		"WebAPIPath":   wasabi.GetWebAPIPath,
		"VEnlOne":      wasabi.GetvEnlOne,
	}
	config.templateSet = make(map[string]*template.Template)

	if err != nil {
		wasabi.Log.Error(err)
	}
	wasabi.Log.Info("Including frontend telegram templates from: ", config.FrontendPath)
	files, err := ioutil.ReadDir(config.FrontendPath)
	if err != nil {
		wasabi.Log.Error(err)
	}

	for _, f := range files {
		lang := f.Name()
		if f.IsDir() && len(lang) == 2 {
			config.templateSet[lang] = template.New("").Funcs(funcMap) // one funcMap for all languages
			// load the masters
			config.templateSet[lang].ParseGlob(config.FrontendPath + "/master/*.tg")
			// overwrite with language specific
			config.templateSet[lang].ParseGlob(config.FrontendPath + "/" + lang + "/*.tg")
			wasabi.Log.Debugf("Templates for lang [%s] %s", lang, config.templateSet[lang].DefinedTemplates())
		}
	}

	return nil
}

func wasabibotTemplateExecute(name, lang string, data interface{}) (string, error) {
	if lang == "" {
		lang = "en"
	}

	_, ok := config.templateSet[lang]
	if !ok {
		lang = "en" // default to english if the map doesn't exist
	}

	// s, _ := json.MarshalIndent(&data, "", "\t")
	// wasabi.Log.Debugf("Calling template %s[%s] with data %s", name, lang, string(s))
	var tpBuffer bytes.Buffer
	if err := config.templateSet[lang].ExecuteTemplate(&tpBuffer, name, data); err != nil {
		wasabi.Log.Notice(err)
		return "", err
	}
	return tpBuffer.String(), nil
}

func wasabibotKeyboards(c *TGConfiguration) error {
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

func wasabibotTeamKeyboard(gid wasabi.GoogleID) (tgbotapi.ReplyKeyboardMarkup, error) {
	var ud wasabi.AgentData
	if err := gid.GetAgentData(&ud); err != nil {
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

		if i > 8 { // too many rows and the screen fills up
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
func wasabibotMessage(msg *tgbotapi.MessageConfig, inMsg *tgbotapi.Update, gid wasabi.GoogleID) error {
	if inMsg.Message.IsCommand() {
		switch inMsg.Message.Command() {
		case "start":
			tmp, _ := wasabibotTemplateExecute("help", inMsg.Message.From.LanguageCode, nil)
			msg.Text = tmp
			msg.ReplyMarkup = config.baseKbd
		case "help":
			tmp, _ := wasabibotTemplateExecute("help", inMsg.Message.From.LanguageCode, nil)
			msg.Text = tmp
			msg.ReplyMarkup = config.baseKbd
		default:
			tmp, _ := wasabibotTemplateExecute("default", inMsg.Message.From.LanguageCode, nil)
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
			tmp, _ := wasabibotTeamKeyboard(gid)
			msg.ReplyMarkup = tmp
			msg.Text = "Teams"
		case "On:":
			msg.Text, _ = wasabibotTemplateExecute("TeamStateChange", inMsg.Message.From.LanguageCode, tStruct{
				State: "On",
				Team:  name,
			})
			gid.SetTeamStateName(name, "On")
			msg.ReplyMarkup, _ = wasabibotTeamKeyboard(gid)
		case "Off:":
			msg.Text, _ = wasabibotTemplateExecute("TeamStateChange", inMsg.Message.From.LanguageCode, tStruct{
				State: "Off",
				Team:  name,
			})
			gid.SetTeamStateName(name, "Off")
			msg.ReplyMarkup, _ = wasabibotTeamKeyboard(gid)
		case "Primary:":
			msg.Text, _ = wasabibotTemplateExecute("TeamStateChange", inMsg.Message.From.LanguageCode, tStruct{
				State: "Primary",
				Team:  name,
			})
			gid.SetTeamStateName(name, "Primary")
			msg.ReplyMarkup, _ = wasabibotTeamKeyboard(gid)
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
		gid.AgentLocation(
			strconv.FormatFloat(inMsg.Message.Location.Latitude, 'f', -1, 64),
			strconv.FormatFloat(inMsg.Message.Location.Longitude, 'f', -1, 64),
			"Telegram",
		)
		msg.ReplyMarkup = config.baseKbd
		msg.Text = "Location Processed"
	}
	return nil
}

// SendMessage is registered with WASABI as a message bus to allow other modules to send messages via Telegram
func SendMessage(gid wasabi.GoogleID, message string) (bool, error) {
	tgid, err := gid.TelegramID()
	if err != nil {
		wasabi.Log.Notice(err)
		return false, err
	}
	tgid64 := int64(tgid)
	if tgid64 == 0 {
		err = fmt.Errorf("TelegramID not found for %s", gid)
		wasabi.Log.Notice(err)
		return false, err
	}
	msg := tgbotapi.NewMessage(tgid64, "")
	msg.Text = message
	msg.ParseMode = "MarkDown"

	bot.Send(msg)
	wasabi.Log.Notice("Sent message to:", gid)
	return true, nil
}

func teammatesNear(gid wasabi.GoogleID, inMsg *tgbotapi.Update) (string, error) {
	var td wasabi.TeamData
	var txt = ""
	maxdistance := 500
	maxresults := 10

	err := gid.TeammatesNear(maxdistance, maxresults, &td)
	if err != nil {
		wasabi.Log.Error(err)
		return txt, err
	}
	txt, err = wasabibotTemplateExecute("Teammates", inMsg.Message.From.LanguageCode, &td)
	if err != nil {
		wasabi.Log.Error(err)
		return txt, err
	}

	return txt, nil
}

func targetsNear(gid wasabi.GoogleID, inMsg *tgbotapi.Update) (string, error) {
	var td wasabi.TeamData
	var txt = ""
	maxdistance := 100
	maxresults := 10

	err := gid.WaypointsNear(maxdistance, maxresults, &td)
	if err != nil {
		wasabi.Log.Error(err)
		return txt, err
	}
	txt, err = wasabibotTemplateExecute("Targets", inMsg.Message.From.LanguageCode, &td)
	if err != nil {
		wasabi.Log.Error(err)
		return txt, err
	}

	return txt, nil
}

func farmsNear(gid wasabi.GoogleID, inMsg *tgbotapi.Update) (string, error) {
	var td wasabi.TeamData
	var txt = ""
	maxdistance := 100
	maxresults := 10

	err := gid.WaypointsNear(maxdistance, maxresults, &td)
	if err != nil {
		wasabi.Log.Error(err)
		return txt, err
	}
	txt, err = wasabibotTemplateExecute("Farms", inMsg.Message.From.LanguageCode, &td)
	if err != nil {
		wasabi.Log.Error(err)
		return txt, err
	}

	return txt, nil
}
