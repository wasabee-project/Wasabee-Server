package wtg

import (
	"context"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/wasabee-project/Wasabee-Server/log"
)

type command struct {
	Command     string
	Description string
	Private     bool
	Group       bool
	Admin       bool
}

var botCommands []command

func setupCommands(ctx context.Context) {
	botCommands = []command{
		{Command: "start", Description: "Initial setup and verification", Private: true},
		{Command: "help", Description: "Show help info", Group: true, Private: true},
		{Command: "status", Description: "Show link status to team/op", Group: true},
		{Command: "assignments", Description: "Show tasks with assignments", Group: true},
		{Command: "unassigned", Description: "Show tasks without assignments", Group: true},
		{Command: "claim", Description: "Claim a task (assign to self)", Group: true},
		{Command: "reject", Description: "Reject a task (remove assignment)", Group: true},
		{Command: "acknowledge", Description: "Acknowledge an assignment", Group: true},
		// Admin only group commands
		{Command: "link", Description: "Link this chat to a Wasabee team", Group: true, Admin: true},
		{Command: "unlink", Description: "Unlink this chat from the Wasabee team", Group: true, Admin: true},
	}

	// 1. Set Global/Private Default Commands
	setScope(ctx, tgbotapi.NewBotCommandScopeAllPrivateChats(), filterCommands(true, false, false))

	// 2. Set Standard Group Commands (for everyone in the group)
	setScope(ctx, tgbotapi.NewBotCommandScopeAllGroupChats(), filterCommands(false, true, false))

	// 3. Set Admin Group Commands (adds link/unlink to their menu)
	setScope(ctx, tgbotapi.NewBotCommandScopeAllChatAdministrators(), filterCommands(false, true, true))
}

// filterCommands selects a subset of our master list based on scope
func filterCommands(private, group, admin bool) []tgbotapi.BotCommand {
	var list []tgbotapi.BotCommand
	for _, c := range botCommands {
		// Matches private scope
		if private && c.Private {
			list = append(list, tgbotapi.BotCommand{Command: c.Command, Description: c.Description})
			continue
		}
		// Matches group scope (if admin is true, show admin commands, else show standard group commands)
		if group && c.Group {
			if c.Admin && !admin {
				continue
			}
			list = append(list, tgbotapi.BotCommand{Command: c.Command, Description: c.Description})
		}
	}
	return list
}

// setScope pushes a command list to a specific Telegram UI scope
func setScope(ctx context.Context, scope tgbotapi.BotCommandScope, cmds []tgbotapi.BotCommand) {
	if len(cmds) == 0 {
		return
	}

	cfg := tgbotapi.NewSetMyCommandsWithScope(scope, cmds...)
	if _, err := bot.Request(cfg); err != nil {
		log.Errorw("failed to set telegram commands for scope", "scope", scope.Type, "error", err)
	}
}
