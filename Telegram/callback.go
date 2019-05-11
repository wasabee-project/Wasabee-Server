package wasabitelegram

import (
	"fmt"
	// "encoding/json"
	"github.com/cloudkucooland/WASABI"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"strconv"
	"strings"
)

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

// callback is where to determine which callback is called, and what to do with it
func callback(update *tgbotapi.Update) (tgbotapi.MessageConfig, error) {
	var resp tgbotapi.APIResponse
	msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
	lang := update.CallbackQuery.Message.From.LanguageCode
	tgid := wasabi.TelegramID(update.CallbackQuery.From.ID)
	gid, _, err := tgid.GidV() // can they be not verified?
	if err != nil {
		wasabi.Log.Error(err)
		return msg, err
	}

	// s, _ := json.MarshalIndent(update.CallbackQuery, "", " " )
	// wasabi.Log.Debug(string(s))

	if update.CallbackQuery.Message.Location != nil && update.CallbackQuery.Message.Location.Latitude != 0 {
		err = gid.AgentLocation(
			strconv.FormatFloat(update.CallbackQuery.Message.Location.Latitude, 'f', -1, 64),
			strconv.FormatFloat(update.CallbackQuery.Message.Location.Longitude, 'f', -1, 64),
			"Telegram",
		)
		if err != nil {
			wasabi.Log.Error(err)
			return msg, err
		}
		msg.Text = "Location Processed"
	}

	if update.CallbackQuery.Message.Chat.Type != "private" {
		wasabi.Log.Errorf("Not in private chat: %s", update.CallbackQuery.Message.Chat.Type)
		return msg, nil
	}

	// data is in format class/action/id e.g. "team/deactivate/wibbly-wobbly-9988"
	command := strings.SplitN(update.CallbackQuery.Data, "/", 3)
	switch command[0] {
	case "team":
		_ = callbackTeam(command[1], command[2], gid, lang, &msg)
		resp, err = bot.AnswerCallbackQuery(
			tgbotapi.CallbackConfig{CallbackQueryID: update.CallbackQuery.ID, Text: "Team Updated", ShowAlert: false},
		)
		tmp, _ := teamKeyboard(gid)
		msg.ReplyMarkup = tmp
	case "operation": // XXX nothing yet
		_ = callbackOperation(command[1], command[2], gid, lang, &msg)
		resp, err = bot.AnswerCallbackQuery(
			tgbotapi.CallbackConfig{CallbackQueryID: update.CallbackQuery.ID, Text: "Operation not supported yet"},
		)
	case "target": // XXX nothing yet
		_ = callbackTarget(command[1], command[2], gid, lang, &msg)
		resp, err = bot.AnswerCallbackQuery(
			tgbotapi.CallbackConfig{CallbackQueryID: update.CallbackQuery.ID, Text: "Target not supported yet"},
		)
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
	name, err := t.Name()
	if err != nil {
		wasabi.Log.Notice(err)
		return err
	}

	switch action {
	case "primary":
		msg.Text, _ = templateExecute("TeamStateChange", lang, tStruct{
			State: "Primary",
			Team:  name,
		})
		err = gid.SetTeamState(t, "Primary")
		if err != nil {
			wasabi.Log.Notice(err)
		}
	case "activate":
		msg.Text, _ = templateExecute("TeamStateChange", lang, tStruct{
			State: "On",
			Team:  name,
		})
		err = gid.SetTeamState(t, "On")
		if err != nil {
			wasabi.Log.Notice(err)
		}
	case "deactivate":
		msg.Text, _ = templateExecute("TeamStateChange", lang, tStruct{
			State: "Off",
			Team:  name,
		})
		err = gid.SetTeamState(t, "Off")
		if err != nil {
			wasabi.Log.Notice(err)
		}
	default:
		err = fmt.Errorf("Unknown team state: %s", action)
		wasabi.Log.Error(err)
		if err != nil {
			wasabi.Log.Notice(err)
		}
	}
	return nil
}

func callbackOperation(action, op string, gid wasabi.GoogleID, lang string, msg *tgbotapi.MessageConfig) error {
	return nil
}

func callbackTarget(action, target string, gid wasabi.GoogleID, lang string, msg *tgbotapi.MessageConfig) error {
	// XXX mark completed, what else?
	return nil
}
