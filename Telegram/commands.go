package wtg

import (
	//"fmt"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"

	//"github.com/wasabee-project/Wasabee-Server/config"
	// "github.com/wasabee-project/Wasabee-Server/generatename"
	"github.com/wasabee-project/Wasabee-Server/log"
	// "github.com/wasabee-project/Wasabee-Server/messaging"
	//"github.com/wasabee-project/Wasabee-Server/model"
	// "github.com/wasabee-project/Wasabee-Server/templates"
)

// need to update tgbotapi to allow ADDDING, not just setting commands for these...
type command struct {
	Command     string
	Aliases     []string // future use
	Description string
	Private     bool // templates and future use
	Group       bool // templates and future use
	Admin       bool // future use
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
	// log.Debugw("setting commands", "command", setmy)
	if res, err := bot.Request(setmy); err != nil {
		log.Errorw(err.Error(), "res", res)
		return
	}
}

/* no way to append, just set ... so only the last one sticks
func broken() {
	active, err := bot.GetMyCommands()
	if err != nil {
		log.Error(err)
	}
	log.Debugw("starting active commands", "active", active)

	for _, c := range commands {
		alreadyactive := false
		for _, a := range active {
			if a.Command == c.command {
				alreadyactive = true
			}
		}
		if alreadyactive {
			continue
		}

		newcmd := tgbotapi.BotCommand{ Command: c.command, Description: c.description}
		setmy := tgbotapi.NewSetMyCommands(newcmd)

		if c.Private && !c.Group {
			bsc := tgbotapi.NewBotCommandScopeAllPrivateChats()
			setmy = tgbotapi.NewSetMyCommandsWithScope(bsc, newcmd)
		}
		if c.Group && !c.Private && !c.Admin {
			bsc := tgbotapi.NewBotCommandScopeAllGroupChats()
			setmy = tgbotapi.NewSetMyCommandsWithScope(bsc, newcmd)
		}
		if c.Group && !c.Private && c.Admin {
			bsc := tgbotapi.NewBotCommandScopeAllChatAdministrators()
			setmy = tgbotapi.NewSetMyCommandsWithScope(bsc, newcmd)
		}
		log.Debugw("setting commands", "command", setmy)
		if res, err := bot.Request(setmy); err != nil {
			log.Errorw(err.Error(), "res", res)
			return
		}
	}

	active, err := bot.GetMyCommands()
	if err != nil {
		log.Error(err)
	}
	log.Debugw("final active commands", "active", active)
} */
