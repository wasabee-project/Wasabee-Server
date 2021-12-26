package wtg

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/rocks"
	"github.com/wasabee-project/Wasabee-Server/templates"
	"github.com/wasabee-project/Wasabee-Server/v"
)

func processDirectMessage(inMsg *tgbotapi.Update) error {
	tgid := model.TelegramID(inMsg.Message.From.ID)
	gid, verified, err := tgid.GidV()
	if err != nil {
		log.Error(err)
		return err
	}

	/* defaultReply, err := templates.ExecuteLang("default", inMsg.Message.From.LanguageCode, nil)
	if err != nil {
		log.Error(err)
		return err
	} */

	// telegram ID is unknown to this server
	if gid == "" {
		log.Infow("unknown user; initializing", "subsystem", "Telegram", "tgusername", inMsg.Message.From.UserName, "tgid", tgid)

		// never logged into this server, check Rocks & V
		fgid, err := firstlogin(tgid, inMsg.Message.From.UserName)
		if fgid != "" && err == nil {
			// firstlogin found something at Rocks (or V lol), use that
			msg := tgbotapi.NewMessage(inMsg.Message.Chat.ID, "")
			msg.ParseMode = "HTML"
			tmp, _ := templates.ExecuteLang("InitTwoSuccess", inMsg.Message.From.LanguageCode, nil)
			msg.Text = tmp
			if _, err = bot.Send(msg); err != nil {
				log.Error(err)
				return err
			}
			return nil
		}

		// start manual assocation process
		msg, err := newUserInit(inMsg)
		if err != nil {
			log.Error(err)
		}
		if _, err = bot.Send(msg); err != nil {
			log.Error(err)
			return err
		}
		return nil
	}

	// verification process started, but not completed
	if !verified {
		log.Infow("verifying Telegram user", "subsystem", "Telegram", "tgusername", inMsg.Message.From.UserName, "tgid", tgid)

		msg, err := newUserVerify(inMsg)
		if err != nil {
			log.Error(err)
		}
		if _, err = bot.Send(msg); err != nil {
			log.Error(err)
			return err
		}
		return nil
	}

	// user is verified, process message
	if err := processMessage(inMsg, gid); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// This is where command processing takes place
func processMessage(inMsg *tgbotapi.Update, gid model.GoogleID) error {
	msg := tgbotapi.NewMessage(inMsg.Message.Chat.ID, "")
	msg.ParseMode = "HTML"

	// update name
	if inMsg.Message.From.UserName != "" {
		tgid := model.TelegramID(inMsg.Message.From.ID)
		if err := tgid.SetName(inMsg.Message.From.UserName); err != nil {
			log.Error(err)
		}
	}

	if inMsg.Message.IsCommand() {
		switch inMsg.Message.Command() {
		// add commands here
		case "start":
			tmp, _ := templates.ExecuteLang("help", inMsg.Message.From.LanguageCode, nil)
			msg.Text = tmp
			msg.ReplyMarkup = baseKbd
		case "help":
			tmp, _ := templates.ExecuteLang("help", inMsg.Message.From.LanguageCode, nil)
			msg.Text = tmp
			msg.ReplyMarkup = baseKbd
		default:
			tmp, _ := templates.ExecuteLang("default", inMsg.Message.From.LanguageCode, nil)
			msg.Text = tmp
			msg.ReplyMarkup = baseKbd
		}
	} else if inMsg.Message.Text != "" {
		switch inMsg.Message.Text {
		// and responses here
		case "wasabee":
			msg.Text = "wasabee rocks"
		default:
			msg.ReplyMarkup = baseKbd
		}
	}

	if inMsg.Message != nil && inMsg.Message.Location != nil {
		log.Debugw("processing location", "subsystem", "Telegram", "GID", gid)
		lat := strconv.FormatFloat(inMsg.Message.Location.Latitude, 'f', -1, 64)
		lon := strconv.FormatFloat(inMsg.Message.Location.Longitude, 'f', -1, 64)
		_ = gid.SetLocation(lat, lon)
	}

	if _, err := bot.Send(msg); err != nil {
		log.Error(err)
		return err
	}

	return nil
}

// checks rocks/v based on tgid, Inits agent if found
func firstlogin(tgid model.TelegramID, name string) (model.GoogleID, error) {
	agent, err := rocks.Search(fmt.Sprint(tgid))
	if err != nil {
		log.Error(err)
		return "", err
	}

	if agent.Gid != "" {
		gid := model.GoogleID(agent.Gid)
		if !gid.Valid() {
			if err := gid.FirstLogin(); err != nil {
				log.Error(err)
				return "", err
			}
		}
		if err := gid.SetTelegramID(tgid, name); err != nil {
			log.Error(err)
			return gid, err
		}
		// rocks success
		return gid, nil
	}

	result, err := v.TelegramSearch(tgid)
	if err != nil {
		log.Error(err)
		return "", err
	}
	if result.Gid != "" {
		log.Debugw("v is so useless")
		result.Gid, _ = model.GetGIDFromEnlID(result.EnlID)
	}

	if result.Gid != "" {
		gid := model.GoogleID(result.Gid)
		if !gid.Valid() {
			if err := gid.FirstLogin(); err != nil {
				log.Error(err)
				return "", err
			}
		}
		if err := gid.SetTelegramID(tgid, name); err != nil {
			log.Error(err)
			return gid, err
		}
		// v success?!
		return gid, nil
	}

	// not found in either service
	return "", nil
}

func newUserInit(inMsg *tgbotapi.Update) (*tgbotapi.MessageConfig, error) {
	msg := tgbotapi.NewMessage(inMsg.Message.Chat.ID, "")
	msg.ParseMode = "HTML"

	var ott model.OneTimeToken
	if inMsg.Message.IsCommand() {
		tokens := strings.Split(inMsg.Message.Text, " ")
		if len(tokens) == 2 {
			ott = model.OneTimeToken(strings.TrimSpace(tokens[1]))
		}
	} else {
		ott = model.OneTimeToken(strings.TrimSpace(inMsg.Message.Text))
	}

	log.Debugw("newUserInit", "text", inMsg.Message.Text)

	tid := model.TelegramID(inMsg.Message.From.ID)
	err := tid.InitAgent(inMsg.Message.From.UserName, ott)
	if err != nil {
		log.Error(err)
		tmp, _ := templates.ExecuteLang("InitOneFail", inMsg.Message.From.LanguageCode, nil)
		msg.Text = tmp
	} else {
		tmp, _ := templates.ExecuteLang("InitOneSuccess", inMsg.Message.From.LanguageCode, nil)
		msg.Text = tmp
	}
	return &msg, err
}

func newUserVerify(inMsg *tgbotapi.Update) (*tgbotapi.MessageConfig, error) {
	msg := tgbotapi.NewMessage(inMsg.Message.Chat.ID, "")
	msg.ParseMode = "HTML"

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
	tid := model.TelegramID(inMsg.Message.From.ID)
	err := tid.VerifyAgent(authtoken)
	if err != nil {
		log.Error(err)
		tmp, _ := templates.ExecuteLang("InitTwoFail", inMsg.Message.From.LanguageCode, nil)
		msg.Text = tmp
	} else {
		tmp, _ := templates.ExecuteLang("InitTwoSuccess", inMsg.Message.From.LanguageCode, nil)
		msg.Text = tmp
	}
	return &msg, err
}
