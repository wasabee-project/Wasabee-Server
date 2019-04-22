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
	baseKbd      tgbotapi.ReplyKeyboardMarkup
	upChan       chan tgbotapi.Update
	hook         string
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
	_ = templates(config.templateSet)
	_ = keyboards(&config)
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

	webroot, _ := wasabi.GetWebroot()
	config.hook = wasabi.GenerateName()
	t := fmt.Sprintf("%s/tg/%s", webroot, config.hook)
	wasabi.Log.Debugf("TG webroot %s", t)
	defer bot.RemoveWebhook()
	_, err = bot.SetWebhook(tgbotapi.NewWebhook(t))
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}

	i := 1
	config.upChan = make(chan tgbotapi.Update, 10) // not using bot.ListenForWebhook() since we need our own bidirectional channel
	for update := range config.upChan {
		err := runUpdate(update)
		if err != nil {
			wasabi.Log.Error(err)
			continue
		}
		if (i % 100) == 0 { // every 100 requests, change the endpoint; I'm _not_ paranoid.
			i = 1
			config.hook = wasabi.GenerateName()
			t = fmt.Sprintf("%s/tg/%s", webroot, config.hook)
			wasabi.Log.Debugf("new TG webroot %s", t)
			_, err = bot.SetWebhook(tgbotapi.NewWebhook(t))
			if err != nil {
				wasabi.Log.Error(err)
			}
		}
		i++
	}
	return nil
}

func runUpdate(update tgbotapi.Update) error {
	var err error
	if update.CallbackQuery != nil {
		msg, err := callback(&update)
		if err != nil {
			wasabi.Log.Error(err)
			return err
		}
		bot.Send(msg)
		bot.DeleteMessage(tgbotapi.NewDeleteMessage(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID))
		return nil
	}

	// callbacks have been processed, it must be a message, right?
	if update.Message == nil {
		return nil
	}

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	defaultReply, err := templateExecute("default", update.Message.From.LanguageCode, nil)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	msg.Text = defaultReply
	msg.ParseMode = "MarkDown"
	// s, _ := json.MarshalIndent(msg, "", "  ")
	// wasabi.Log.Debug(string(s))

	tgid := wasabi.TelegramID(update.Message.From.ID)
	gid, verified, err := tgid.GidV()
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}

	if gid == "" {
		wasabi.Log.Debugf("unknown user: %s (%s); initializing", update.Message.From.UserName, string(update.Message.From.ID))
		fgid, err := runRocks(tgid)
		if fgid != "" && err == nil {
			tmp, _ := templateExecute("InitTwoSuccess", update.Message.From.LanguageCode, nil)
			msg.Text = tmp
		} else {
			err = newUserInit(&msg, &update)
			if err != nil {
				wasabi.Log.Error(err)
			}
		}
	} else if !verified {
		wasabi.Log.Debugf("unverified user: %s (%s); verifying", update.Message.From.UserName, string(update.Message.From.ID))
		err = newUserVerify(&msg, &update)
		if err != nil {
			wasabi.Log.Error(err)
		}
	} else { // verified user, process message
		if err = message(&msg, &update, gid); err != nil {
			wasabi.Log.Error(err)
		}
	}

	bot.Send(msg)
	bot.DeleteMessage(tgbotapi.NewDeleteMessage(update.Message.Chat.ID, update.Message.MessageID))

	return nil
}

func newUserInit(msg *tgbotapi.MessageConfig, inMsg *tgbotapi.Update) error {
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
		tmp, _ := templateExecute("InitOneFail", inMsg.Message.From.LanguageCode, nil)
		msg.Text = tmp
	} else {
		tmp, _ := templateExecute("InitOneSuccess", inMsg.Message.From.LanguageCode, nil)
		msg.Text = tmp
	}
	return err
}

func newUserVerify(msg *tgbotapi.MessageConfig, inMsg *tgbotapi.Update) error {
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
		tmp, _ := templateExecute("InitTwoFail", inMsg.Message.From.LanguageCode, nil)
		msg.Text = tmp
	} else {
		tmp, _ := templateExecute("InitTwoSuccess", inMsg.Message.From.LanguageCode, nil)
		msg.Text = tmp
	}
	return err
}

func templates(t map[string]*template.Template) error {
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

func templateExecute(name, lang string, data interface{}) (string, error) {
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

func keyboards(c *TGConfiguration) error {
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

func teamKeyboard(gid wasabi.GoogleID) (tgbotapi.InlineKeyboardMarkup, error) {
	var ud wasabi.AgentData
	var rows [][]tgbotapi.InlineKeyboardButton

	if err := gid.GetAgentData(&ud); err == nil {
		var i int
		for _, v := range ud.Teams {
			i++
			var row []tgbotapi.InlineKeyboardButton
			var on, off, primary tgbotapi.InlineKeyboardButton
			if v.State == "Off" {
				on = tgbotapi.NewInlineKeyboardButtonData("Activate "+v.Name, "team/activate/"+v.ID)
				row = append(row, on)
			}
			if v.State == "On" || v.State == "Primary" {
				off = tgbotapi.NewInlineKeyboardButtonData("Deactivate "+v.Name, "team/deactivate/"+v.ID)
				row = append(row, off)
			}
			if v.State == "On" {
				primary = tgbotapi.NewInlineKeyboardButtonData("Make "+v.Name+" Primary", "team/primary/"+v.ID)
				row = append(row, primary)
			}
			rows = append(rows, row)

			if i > 8 { // too many rows and the screen fills up
				break
			}
		}
	}

	tmp := tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: rows,
	}
	return tmp, nil
}

// This is where command processing takes place
func message(msg *tgbotapi.MessageConfig, inMsg *tgbotapi.Update, gid wasabi.GoogleID) error {
	if inMsg.Message.IsCommand() {
		switch inMsg.Message.Command() {
		case "start":
			tmp, _ := templateExecute("help", inMsg.Message.From.LanguageCode, nil)
			msg.Text = tmp
			msg.ReplyMarkup = config.baseKbd
		case "help":
			tmp, _ := templateExecute("help", inMsg.Message.From.LanguageCode, nil)
			msg.Text = tmp
			msg.ReplyMarkup = config.baseKbd
		default:
			tmp, _ := templateExecute("default", inMsg.Message.From.LanguageCode, nil)
			msg.Text = tmp
			msg.ReplyMarkup = config.baseKbd
		}
	} else if inMsg.Message.Text != "" {
		messageText(msg, inMsg, gid)
	}

	if inMsg.Message.Location != nil {
		gid.AgentLocation(
			strconv.FormatFloat(inMsg.Message.Location.Latitude, 'f', -1, 64),
			strconv.FormatFloat(inMsg.Message.Location.Longitude, 'f', -1, 64),
			"Telegram",
		)
		msg.Text = "Location Processed"
	}

	return nil
}

// callback is where to determine which callback is called, and what to do with it
func callback(update *tgbotapi.Update) (tgbotapi.MessageConfig, error) {
	msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
	tgid := wasabi.TelegramID(update.CallbackQuery.From.ID)
	gid, _, err := tgid.GidV()
	if err != nil {
		wasabi.Log.Error(err)
		return msg, err
	}

	// s, _ := json.MarshalIndent(update.CallbackQuery, "", " " )
	// wasabi.Log.Debug(string(s))

	var resp tgbotapi.APIResponse
	if update.CallbackQuery.Message.Chat.Type != "private" {
		wasabi.Log.Errorf("Not in private chat: %s", update.CallbackQuery.Message.Chat.Type)
		return msg, nil
	}
	lang := update.CallbackQuery.Message.From.LanguageCode

	command := strings.SplitN(update.CallbackQuery.Data, "/", 3)
	switch command[0] {
	case "team":
		_ = callbackTeam(command[1], command[2], gid, lang, &msg)
		resp, err = bot.AnswerCallbackQuery(
			tgbotapi.CallbackConfig{CallbackQueryID: update.CallbackQuery.ID, Text: "Team Updated", ShowAlert: false},
		)
		tmp, _ := teamKeyboard(gid)
		msg.ReplyMarkup = tmp
	default:
		resp, err = bot.AnswerCallbackQuery(
			tgbotapi.CallbackConfig{CallbackQueryID: update.CallbackQuery.ID, Text: "Unknown Callback"},
		)
	}

	if err != nil {
		wasabi.Log.Error(err)
		return msg, err
	}
	if !resp.Ok {
		wasabi.Log.Error(resp.Description)
	}
	return msg, nil
}

func callbackTeam(action, team string, gid wasabi.GoogleID, lang string, msg *tgbotapi.MessageConfig) error {
	type tStruct struct {
		State string
		Team  string
	}

	t := wasabi.TeamID(team)
	name, _ := t.Name()

	switch action {
	case "primary":
		msg.Text, _ = templateExecute("TeamStateChange", lang, tStruct{
			State: "Primary",
			Team:  name,
		})
		gid.SetTeamState(t, "Primary")
	case "activate":
		msg.Text, _ = templateExecute("TeamStateChange", lang, tStruct{
			State: "On",
			Team:  name,
		})
		gid.SetTeamState(t, "On")
	case "deactivate":
		msg.Text, _ = templateExecute("TeamStateChange", lang, tStruct{
			State: "Off",
			Team:  name,
		})
		gid.SetTeamState(t, "Off")
	default:
		wasabi.Log.Error("Unknown Team command")
	}
	return nil
}

func messageText(msg *tgbotapi.MessageConfig, inMsg *tgbotapi.Update, gid wasabi.GoogleID) {
	// get first word
	tokens := strings.Split(inMsg.Message.Text, " ")
	cmd := tokens[0]

	switch cmd {
	case "Teams":
		tmp, _ := teamKeyboard(gid)
		msg.ReplyMarkup = tmp
		msg.Text = "Your Teams"
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
	txt, err = templateExecute("Teammates", inMsg.Message.From.LanguageCode, &td)
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
	txt, err = templateExecute("Targets", inMsg.Message.From.LanguageCode, &td)
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
	txt, err = templateExecute("Farms", inMsg.Message.From.LanguageCode, &td)
	if err != nil {
		wasabi.Log.Error(err)
		return txt, err
	}

	return txt, nil
}

// checks rocks based on tgid, Inits agent if found
func runRocks(tgid wasabi.TelegramID) (wasabi.GoogleID, error) {
	var agent wasabi.RocksAgent

	err := tgid.RocksSearch(&agent)
	if err != nil {
		wasabi.Log.Error(err)
		return "", err
	}
	if agent.Gid == "" {
		return "", nil
	}
	_, err = (agent.Gid).InitAgent()
	if err != nil {
		wasabi.Log.Error(err)
		return agent.Gid, err
	}
	return agent.Gid, nil
}
