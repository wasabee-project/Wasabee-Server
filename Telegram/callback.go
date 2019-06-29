package wasabitelegram

import (
	"fmt"
	// "encoding/json"
	"github.com/cloudkucooland/WASABI"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"strconv"
	"strings"
)

func teamKeyboard(gid wasabi.GoogleID) tgbotapi.InlineKeyboardMarkup {
	var ud wasabi.AgentData
	var rows [][]tgbotapi.InlineKeyboardButton

	if err := gid.GetAgentData(&ud); err == nil {
		var i int
		for _, v := range ud.Teams {
			i++
			var row []tgbotapi.InlineKeyboardButton
			var on, off tgbotapi.InlineKeyboardButton
			if v.State == "Off" {
				on = tgbotapi.NewInlineKeyboardButtonData("Activate "+v.Name, "team/activate/"+v.ID)
				row = append(row, on)
			}
			if v.State == "On" {
				off = tgbotapi.NewInlineKeyboardButtonData("Deactivate "+v.Name, "team/deactivate/"+v.ID)
				row = append(row, off)
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
	return tmp
}

func assignmentKeyboard(gid wasabi.GoogleID) tgbotapi.InlineKeyboardMarkup {
	var ud wasabi.AgentData
	var rows [][]tgbotapi.InlineKeyboardButton
	var a wasabi.Assignments

	if err := gid.GetAgentData(&ud); err == nil {
		for _, op := range ud.Assignments {
			err = gid.Assignments(op.OpID, &a)
			if err != nil {
				wasabi.Log.Error(err)
				continue
			}
			for _, marker := range a.Markers {
				var row []tgbotapi.InlineKeyboardButton
				var action, reject tgbotapi.InlineKeyboardButton
				title := fmt.Sprintf("%s %s - Complete", marker.Type, a.Portals[marker.PortalID].Name)
				cmd := fmt.Sprintf("marker/complete/%s", marker.ID)
				rcmd := fmt.Sprintf("marker/reject/%s", marker.ID)
				action = tgbotapi.NewInlineKeyboardButtonData(title, cmd)
				reject = tgbotapi.NewInlineKeyboardButtonData("reject", rcmd)
				row = append(row, action)
				row = append(row, reject)
				rows = append(rows, row)
			}
		}
	}

	tmp := tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: rows,
	}
	return tmp
}

// callback is where to determine which callback is called, and what to do with it
func callback(update *tgbotapi.Update) (tgbotapi.MessageConfig, error) {
	var resp tgbotapi.APIResponse
	msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
	lang := update.CallbackQuery.Message.From.LanguageCode
	gid, err := wasabi.TelegramID(update.CallbackQuery.From.ID).Gid()
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
		tmp := teamKeyboard(gid)
		msg.ReplyMarkup = tmp
	case "operation": // XXX nothing yet
		_ = callbackOperation(command[1], command[2], gid, lang, &msg)
		resp, err = bot.AnswerCallbackQuery(
			tgbotapi.CallbackConfig{CallbackQueryID: update.CallbackQuery.ID, Text: "Operation not supported yet"},
		)
	case "marker": // XXX nothing yet
		_ = callbackMarker(command[1], command[2], gid, lang, &msg)
		resp, err = bot.AnswerCallbackQuery(
			tgbotapi.CallbackConfig{CallbackQueryID: update.CallbackQuery.ID, Text: "Marker Updated"},
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
		err = fmt.Errorf("unknown team state: %s", action)
		wasabi.Log.Info(err)
	}
	return nil
}

func callbackOperation(action, op string, gid wasabi.GoogleID, lang string, msg *tgbotapi.MessageConfig) error {
	return nil
}

func callbackMarker(action, target string, gid wasabi.GoogleID, lang string, msg *tgbotapi.MessageConfig) error {
	switch action {
	case "complete":
		msg.Text = "assignment completion coming soon"
	case "reject":
		msg.Text = "assignment rejection coming soon"
	default:
		err := fmt.Errorf("unknown marker action: %s", action)
		wasabi.Log.Info(err)
	}
	return nil
}
