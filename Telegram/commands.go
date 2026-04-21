package wtg

import (
	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/wasabee-project/Wasabee-Server/log"
)

type command struct {
	Command     string
	Description string
	Aliases     []string // future use
	Private     bool     // templates and future use
	Group       bool     // templates and future use
	Admin       bool     // future use
}

var commands []command

func setupCommands() {
	commands = []command{
		{Command: "unlink", Description: "unlink this team from the team/op", Group: true, Admin: true},
		{Command: "link", Description: "link this team to the team/op", Group: true, Admin: true},
		{Command: "status", Description: "show the status of any link to a team/op", Group: true},
		{Command: "assignments", Description: "show tasks with assignments", Group: true},
		{Command: "unassigned", Description: "show tasks without assignments", Group: true},
		{Command: "claim", Description: "claim a task (assign to self)", Group: true},
		{Command: "reject", Description: "reject a task (remove assignment)", Group: true},
		{Command: "acknowledge", Description: "acknowledge an assignment", Group: true},
		{Command: "start", Description: "initial setup and grant bot permission to communicate", Private: true},
		{Command: "help", Description: "show help info", Group: true, Private: true},
	}

	desired := []tgbotapi.BotCommand{}
	for _, c := range commands {
		newcmd := tgbotapi.BotCommand{Command: c.Command, Description: c.Description}
		desired = append(desired, newcmd)
	}

	setmy := tgbotapi.NewSetMyCommands(desired...)
	if res, err := bot.Request(setmy); err != nil {
		log.Errorw(err.Error(), "res", res)
		return
	}
}
