package wasabeetelegram

import (
	"strconv"

	"github.com/go-telegram-bot-api/telegram-bot-api"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

func processDirectMessage(inMsg *tgbotapi.Update) error {
	tgid := model.TelegramID(inMsg.Message.From.ID)
	gid, verified, err := tgid.GidV()
	if err != nil {
		log.Error(err)
		return err
	}

	msg := tgbotapi.NewMessage(inMsg.Message.Chat.ID, "")
	defaultReply, err := templateExecute("default", inMsg.Message.From.LanguageCode, nil)
	if err != nil {
		log.Error(err)
		return err
	}
	msg.Text = defaultReply
	msg.ParseMode = "HTML"

	if gid == "" {
		log.Infow("unknown user; initializing", "subsystem", "Telegram", "tgusername", inMsg.Message.From.UserName, "tgid", tgid)
		fgid, err := runRocks(tgid)
		if fgid != "" && err == nil {
			tmp, _ := templateExecute("InitTwoSuccess", inMsg.Message.From.LanguageCode, nil)
			msg.Text = tmp
		} else {
			if err = newUserInit(&msg, inMsg); err != nil {
				log.Error(err)
			}
		}
		if _, err = bot.Send(msg); err != nil {
			log.Error(err)
			return err
		}
		return nil
	}

	if !verified {
		log.Infow("verifying Telegram user", "subsystem", "Telegram", "tgusername", inMsg.Message.From.UserName, "tgid", tgid)
		if err = newUserVerify(&msg, inMsg); err != nil {
			log.Error(err)
		}
		if _, err = bot.Send(msg); err != nil {
			log.Error(err)
			return err
		}
		return nil
	}

	// user is verified, process message
	if err := processMessage(&msg, inMsg, gid); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// This is where command processing takes place
func processMessage(msg *tgbotapi.MessageConfig, inMsg *tgbotapi.Update, gid model.GoogleID) error {
	// we don't get the name from the agent when verified via rocks, go ahead and update
	if inMsg.Message.From.UserName != "" {
		tgid := model.TelegramID(inMsg.Message.From.ID)
		if err := tgid.UpdateName(inMsg.Message.From.UserName); err != nil {
			log.Error(err)
		}
	}
	if inMsg.Message.IsCommand() {
		switch inMsg.Message.Command() {
		case "start":
			tmp, _ := templateExecute("help", inMsg.Message.From.LanguageCode, nil)
			msg.Text = tmp
			msg.ReplyMarkup = c.baseKbd
		case "help":
			tmp, _ := templateExecute("help", inMsg.Message.From.LanguageCode, nil)
			msg.Text = tmp
			msg.ReplyMarkup = c.baseKbd
		default:
			tmp, _ := templateExecute("default", inMsg.Message.From.LanguageCode, nil)
			msg.Text = tmp
			msg.ReplyMarkup = c.baseKbd
		}

		if _, err := bot.DeleteMessage(tgbotapi.NewDeleteMessage(inMsg.Message.Chat.ID, inMsg.Message.MessageID)); err != nil {
			log.Error(err)
			return err
		}
	} else if inMsg.Message.Text != "" {
		switch inMsg.Message.Text {
		case "Teams":
			msg.ReplyMarkup = teamKeyboard(gid)
			msg.Text = "Your Teams"
		default:
			msg.ReplyMarkup = c.baseKbd
		}
	}

	if inMsg.Message != nil && inMsg.Message.Location != nil {
		log.Debugw("processing location", "subsystem", "Telegram", "GID", gid)
		lat := strconv.FormatFloat(inMsg.Message.Location.Latitude, 'f', -1, 64)
		lon := strconv.FormatFloat(inMsg.Message.Location.Longitude, 'f', -1, 64)
		_ = gid.AgentLocation(lat, lon)
		//XXX  gid.PSLocation(lat, lon)
	}

	if _, err := bot.Send(msg); err != nil {
		log.Error(err)
		return err
	}

	return nil
}
