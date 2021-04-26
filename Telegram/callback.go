package wasabeetelegram

import (
	"fmt"
	// "encoding/json"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/wasabee-project/Wasabee-Server"
	"strconv"
	"strings"
)

func teamKeyboard(gid wasabee.GoogleID) tgbotapi.InlineKeyboardMarkup {
	var ud wasabee.AgentData
	var rows [][]tgbotapi.InlineKeyboardButton

	if err := gid.GetAgentData(&ud); err == nil {
		var i int
		for _, v := range ud.Teams {
			i++
			var row []tgbotapi.InlineKeyboardButton
			var on, off tgbotapi.InlineKeyboardButton
			if v.State == "Off" {
				on = tgbotapi.NewInlineKeyboardButtonData("Activate "+v.Name, "team/activate/"+v.ID.String())
				row = append(row, on)
			}
			if v.State == "On" {
				off = tgbotapi.NewInlineKeyboardButtonData("Deactivate "+v.Name, "team/deactivate/"+v.ID.String())
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

// callback is where to determine which callback is called, and what to do with it
func callback(update *tgbotapi.Update) (tgbotapi.MessageConfig, error) {
	var resp tgbotapi.APIResponse
	msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
	lang := update.CallbackQuery.Message.From.LanguageCode
	gid, err := wasabee.TelegramID(update.CallbackQuery.From.ID).Gid()
	if err != nil {
		wasabee.Log.Error(err)
		return msg, err
	}

	// s, _ := json.MarshalIndent(update.CallbackQuery, "", " " )
	// wasabee.Log.Debug(string(s))

	if update.CallbackQuery.Message.Location != nil && update.CallbackQuery.Message.Location.Latitude != 0 {
		lat := strconv.FormatFloat(update.CallbackQuery.Message.Location.Latitude, 'f', -1, 64)
		lon := strconv.FormatFloat(update.CallbackQuery.Message.Location.Longitude, 'f', -1, 64)
		err = gid.AgentLocation(lat, lon)
		if err != nil {
			wasabee.Log.Error(err)
			return msg, err
		}
		msg.Text = "Location Processed"
		gid.PSLocation(lat, lon)
	}

	if update.CallbackQuery.Message.Chat.Type != "private" {
		wasabee.Log.Errorf("Not in private chat: %s", update.CallbackQuery.Message.Chat.Type)
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
		msg.ReplyMarkup = teamKeyboard(gid)
	default:
		resp, err = bot.AnswerCallbackQuery(
			tgbotapi.CallbackConfig{CallbackQueryID: update.CallbackQuery.ID, Text: "Unknown Callback"},
		)
	}

	if err != nil {
		wasabee.Log.Error(err)
		return msg, err
	}
	if !resp.Ok {
		wasabee.Log.Error(resp.Description)
	}
	return msg, nil
}

func callbackTeam(action, team string, gid wasabee.GoogleID, lang string, msg *tgbotapi.MessageConfig) error {
	type tStruct struct {
		State string
		Team  string
	}

	t := wasabee.TeamID(team)
	name, err := t.Name()
	if err != nil {
		wasabee.Log.Error(err)
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
			wasabee.Log.Error(err)
		}
	case "deactivate":
		msg.Text, _ = templateExecute("TeamStateChange", lang, tStruct{
			State: "Off",
			Team:  name,
		})
		err = gid.SetTeamState(t, "Off")
		if err != nil {
			wasabee.Log.Error(err)
		}
	default:
		err = fmt.Errorf("unknown team state: %s", action)
		wasabee.Log.Error(err)
	}
	return nil
}
