package wtg

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/templates"
)

func processChatMessage(ctx context.Context, inMsg *tgbotapi.Update) error {
	tgid := model.TelegramID(inMsg.Message.From.ID)
	chatID := model.TelegramID(inMsg.Message.Chat.ID)

	// Sync local cache of chat members
	if !model.IsChatMember(ctx, tgid, chatID) {
		if err := model.AddToChatMemberList(ctx, tgid, chatID); err != nil {
			log.Debug(err)
			text, _ := templates.ExecuteLang("agentUnknown", inMsg.Message.From.LanguageCode, inMsg.Message.From.UserName)
			sendQueue <- tgbotapi.NewMessage(int64(chatID), text)
		}
	}

	if inMsg.Message.IsCommand() {
		return processChatCommand(ctx, inMsg)
	}
	return chatResponses(ctx, inMsg)
}

func processChatCommand(ctx context.Context, inMsg *tgbotapi.Update) error {
	switch inMsg.Message.Command() {
	case "unlink":
		gcUnlink(ctx, inMsg)
	case "link":
		gcLink(ctx, inMsg)
	case "status":
		gcStatus(ctx, inMsg)
	case "assignments", "assigned":
		gcAssigned(ctx, inMsg)
	case "unassigned":
		gcUnassigned(ctx, inMsg)
	case "claim":
		gcTaskAction(ctx, inMsg, "claim")
	case "acknowledge":
		gcTaskAction(ctx, inMsg, "acknowledge")
	case "reject":
		gcTaskAction(ctx, inMsg, "reject")
	default:
		text, _ := templates.ExecuteLang("default", inMsg.Message.From.LanguageCode, botCommands)
		msg := tgbotapi.NewMessage(inMsg.Message.Chat.ID, text)
		msg.ParseMode = "HTML"
		sendQueue <- msg
	}
	return nil
}

func chatResponses(ctx context.Context, inMsg *tgbotapi.Update) error {
	teamID, _, err := model.ChatToTeam(ctx, inMsg.Message.Chat.ID)
	if err != nil {
		return nil // Not a linked chat
	}

	// Bot removed from group
	if inMsg.Message.LeftChatMember != nil && inMsg.Message.LeftChatMember.ID == bot.Self.ID {
		return teamID.UnlinkFromTelegramChat(ctx)
	}

	// New members joined: Add them to the Wasabee team if they are verified agents
	if inMsg.Message.NewChatMembers != nil {
		for _, user := range inMsg.Message.NewChatMembers {
			tgid := model.TelegramID(user.ID)
			gid, err := tgid.Gid(ctx)
			if err != nil {
				continue
			}
			_ = tgid.SetName(ctx, user.UserName)
			_ = teamID.AddAgent(ctx, gid)
		}
	}

	// Members left/kicked: Remove from Wasabee team
	if inMsg.Message.LeftChatMember != nil {
		tgid := model.TelegramID(inMsg.Message.LeftChatMember.ID)
		if gid, err := tgid.Gid(ctx); err == nil {
			_ = teamID.RemoveAgent(ctx, gid)
		}
	}
	return nil
}

func liveLocationUpdate(ctx context.Context, inMsg *tgbotapi.Update) error {
	tgid := model.TelegramID(inMsg.EditedMessage.From.ID)
	gid, verified, err := tgid.GidV(ctx)
	if err != nil || !verified || gid == "" {
		return err
	}

	lat := strconv.FormatFloat(inMsg.EditedMessage.Location.Latitude, 'f', -1, 64)
	lon := strconv.FormatFloat(inMsg.EditedMessage.Location.Longitude, 'f', -1, 64)
	return gid.SetLocation(ctx, lat, lon)
}

func addToChat(ctx context.Context, g messaging.GoogleID, t messaging.TeamID) error {
	gid := model.GoogleID(g)
	teamID := model.TeamID(t)

	chatID, err := teamID.TelegramChat(ctx)
	if err != nil || chatID == 0 {
		return err
	}

	tgid, err := gid.TelegramID(ctx)
	if err != nil || tgid == 0 {
		return nil // Agent hasn't linked TG yet
	}

	// Create a one-time use invite link for the agent
	teamname, _ := teamID.Name(ctx)
	if err := sendInviteLink(ctx, tgid, chatID, teamname); err != nil {
		log.Errorw("failed to send invite link", "tgid", tgid, "error", err)
	}

	return model.AddToChatMemberList(ctx, tgid, model.TelegramID(chatID))
}

func sendInviteLink(ctx context.Context, tgid model.TelegramID, chatID int64, teamName string) error {
	// Create invite link with 1-use limit
	config := tgbotapi.CreateChatInviteLinkConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: chatID,
		},
		MemberLimit: 1,
	}

	resp, err := bot.Request(config)
	if err != nil {
		return err
	}

	var r struct {
		Link string `json:"invite_link"`
	}
	_ = json.Unmarshal(resp.Result, &r)

	text, _ := templates.ExecuteLang("invitedToTeam", "en", struct {
		TeamName string
		Link     string
	}{TeamName: teamName, Link: r.Link})

	msg := tgbotapi.NewMessage(int64(tgid), text)
	msg.ParseMode = "HTML"
	sendQueue <- msg
	return nil
}

func removeFromChat(ctx context.Context, g messaging.GoogleID, t messaging.TeamID) error {
	gid := model.GoogleID(g)
	teamID := model.TeamID(t)

	chatID, err := teamID.TelegramChat(ctx)
	if err != nil || chatID == 0 {
		return err
	}

	tgid, err := gid.TelegramID(ctx)
	if err != nil || tgid == 0 {
		return nil
	}

	// Kick from chat (Ban for 30s then unban to mimic a kick)
	kick := tgbotapi.BanChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: chatID,
			UserID: int64(tgid),
		},
		UntilDate: time.Now().Add(35 * time.Second).Unix(),
	}

	_, err = bot.Request(kick)
	_ = model.RemoveFromChatMemberList(ctx, tgid, model.TelegramID(chatID))

	return err
}
