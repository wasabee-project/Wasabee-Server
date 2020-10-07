package wasabeetelegram

import (
	"strconv"

	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/wasabee-project/Wasabee-Server"
)

func processDirectMessage(inMsg *tgbotapi.Update) error {
	tgid := wasabee.TelegramID(inMsg.Message.From.ID)
	gid, verified, err := tgid.GidV()
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}

	msg := tgbotapi.NewMessage(inMsg.Message.Chat.ID, "")
	defaultReply, err := templateExecute("default", inMsg.Message.From.LanguageCode, nil)
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	msg.Text = defaultReply
	msg.ParseMode = "HTML"

	if gid == "" {
		wasabee.Log.Infow("unknown user; initializing", "subsystem", "Telegram", "tgusername", inMsg.Message.From.UserName, "tgid", tgid)
		fgid, err := runRocks(tgid)
		if fgid != "" && err == nil {
			tmp, _ := templateExecute("InitTwoSuccess", inMsg.Message.From.LanguageCode, nil)
			msg.Text = tmp
		} else {
			err = newUserInit(&msg, inMsg)
			if err != nil {
				wasabee.Log.Error(err)
			}
		}
		if _, err = bot.Send(msg); err != nil {
			wasabee.Log.Error(err)
			return err
		}
		return nil
	}

	if !verified {
		wasabee.Log.Infow("verifying Telegram user", "subsystem", "Telegram", "tgusername", inMsg.Message.From.UserName, "tgid", tgid)
		err = newUserVerify(&msg, inMsg)
		if err != nil {
			wasabee.Log.Error(err)
		}
		if _, err = bot.Send(msg); err != nil {
			wasabee.Log.Error(err)
			return err
		}
		return nil
	}

	// verified user, process message
	if err := processMessage(&msg, inMsg, gid); err != nil {
		wasabee.Log.Error(err)
		return err
	}
	return nil
}

// This is where command processing takes place
func processMessage(msg *tgbotapi.MessageConfig, inMsg *tgbotapi.Update, gid wasabee.GoogleID) error {
	// kludge to undo a mistake I made by ignoring this data for the past year
	if inMsg.Message.From.UserName != "" {
		tgid := wasabee.TelegramID(inMsg.Message.From.ID)
		if err := tgid.UpdateName(inMsg.Message.From.UserName); err != nil {
			wasabee.Log.Error(err)
		}
	}
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

		if _, err := bot.DeleteMessage(tgbotapi.NewDeleteMessage(inMsg.Message.Chat.ID, inMsg.Message.MessageID)); err != nil {
			wasabee.Log.Error(err)
			return err
		}
	} else if inMsg.Message.Text != "" {
		switch inMsg.Message.Text {
		/* case "My Assignments":
			msg.ReplyMarkup = assignmentKeyboard(gid)
			msg.Text = "My Assignments"
		case "Nearby Tasks":
			msg.ReplyMarkup = nearbyAssignmentKeyboard(gid)
			msg.Text = "Nearby Tasks" */
		case "Teams":
			msg.ReplyMarkup = teamKeyboard(gid)
			msg.Text = "Your Teams"
		case "Teammates Nearby":
			tmp, _ := teammatesNear(gid, inMsg)
			msg.Text = tmp
			msg.ReplyMarkup = config.baseKbd
			msg.DisableWebPagePreview = true
		default:
			msg.ReplyMarkup = config.baseKbd
		}
	}

	if inMsg.Message != nil && inMsg.Message.Location != nil {
		wasabee.Log.Debugw("processing location", "subsystem", "Telegram", "GID", gid)
		lat := strconv.FormatFloat(inMsg.Message.Location.Latitude, 'f', -1, 64)
		lon := strconv.FormatFloat(inMsg.Message.Location.Longitude, 'f', -1, 64)
		_ = gid.AgentLocation(lat, lon)
		gid.PSLocation(lat, lon)
	}

	if _, err := bot.Send(msg); err != nil {
		wasabee.Log.Error(err)
		return err
	}

	return nil
}

func teammatesNear(gid wasabee.GoogleID, inMsg *tgbotapi.Update) (string, error) {
	var td wasabee.TeamData
	var txt = ""
	maxdistance := 500
	maxresults := 10

	err := gid.TeammatesNear(maxdistance, maxresults, &td)
	if err != nil {
		wasabee.Log.Error(err)
		return txt, err
	}
	txt, err = templateExecute("Teammates", inMsg.Message.From.LanguageCode, &td)
	if err != nil {
		wasabee.Log.Error(err)
		return txt, err
	}

	return txt, nil
}
