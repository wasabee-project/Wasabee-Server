package wtg

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

// callback is where to determine which callback is called, and what to do with it
func callback(ctx context.Context, update *tgbotapi.Update) (tgbotapi.Chattable, error) {
	log.Debugw("callback", "subsystem", "Telegram", "query", update.CallbackQuery.Data)

	// Identify the agent
	tgid := model.TelegramID(update.CallbackQuery.From.ID)
	gid, err := tgid.Gid(ctx)
	if err != nil || gid == "" {
		// Answer the callback so the UI doesn't hang, even on error
		bot.Request(tgbotapi.NewCallbackWithAlert(update.CallbackQuery.ID, "Agent not verified"))
		return nil, err
	}

	// Always acknowledge the callback query immediately to stop the loading spinner
	// We can update the message text or show an alert later if needed.
	callbackACK := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
	if _, err := bot.Request(callbackACK); err != nil {
		log.Error(err)
	}

	// Data is in format class/action/id e.g. "team/deactivate/wibbly-wobbly-9988"
	command := strings.SplitN(update.CallbackQuery.Data, "/", 3)
	if len(command) < 1 {
		return nil, fmt.Errorf("callback without command")
	}

	// Prepare a response message if necessary
	msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")

	switch command[0] {
	case "wasabee":
		msg.Text = "wasabee rocks"
	case "task":
		// This is where task ack/complete logic will go
		// msg.Text = handleTaskCallback(ctx, gid, command[1:])
		msg.Text = "Task updates not implemented yet"
	case "team":
		// msg.Text = handleTeamCallback(ctx, gid, command[1:])
		msg.Text = "Team updates not implemented yet"
	default:
		log.Debugw("unknown callback", "command", command[0])
		return nil, nil
	}

	// If we didn't set any text, don't send a new message
	if msg.Text == "" {
		return nil, nil
	}

	return msg, nil
}
