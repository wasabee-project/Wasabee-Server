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
	command     string
	aliases     []string // future use
	description string
	private     bool // future use
	group       bool // future use
	admin       bool // future use
}

var commands []command

func setupCommands() {
	commands = []command{
		{command: "unlink", description: "unlink this team from the team/op", group: true, admin: true},
		{command: "link", description: "link this team to the team/op", group: true, admin: true},
		{command: "status", description: "show the status of any link to a team/op", group: true},
		{command: "assignments", description: "show tasks with assignments", group: true},
		{command: "unassigned", description: "show tasks without assignments", group: true},
		{command: "claim", description: "claim a task (assign to self)", group: true},
		{command: "reject", description: "reject a task (remove assignment)", group: true},
		{command: "acknowledge", description: "acknowledge an assignment", group: true},
		{command: "start", description: "inital setup and grant bot permission to communicate", private: true},
		{command: "help", description: "show help info", group: true, private: true},
	}

	desired := []tgbotapi.BotCommand{}
	for _, c := range commands {
		newcmd := tgbotapi.BotCommand{Command: c.command, Description: c.description}
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

		if c.private && !c.group {
			bsc := tgbotapi.NewBotCommandScopeAllPrivateChats()
			setmy = tgbotapi.NewSetMyCommandsWithScope(bsc, newcmd)
		}
		if c.group && !c.private && !c.admin {
			bsc := tgbotapi.NewBotCommandScopeAllGroupChats()
			setmy = tgbotapi.NewSetMyCommandsWithScope(bsc, newcmd)
		}
		if c.group && !c.private && c.admin {
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
