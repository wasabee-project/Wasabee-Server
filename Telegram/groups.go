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
	fromID := model.TelegramID(inMsg.Message.From.ID)
	chatID := model.TelegramID(inMsg.Message.Chat.ID)

	// Keep our internal mapping of who is in which chat up to date
	if !model.IsChatMember(ctx, fromID, chatID) {
		log.Debugw("adding agent to chat list", "agent", fromID, "chat", chatID)
		if err := model.AddToChatMemberList(ctx, fromID, chatID); err != nil {
			log.Error(err)
			text, _ := templates.ExecuteLang("agentUnknown", inMsg.Message.From.LanguageCode, inMsg.Message.From.UserName)
			msg := tgbotapi.NewMessage(inMsg.Message.Chat.ID, text)
			sendQueue <- msg
		}
	}

	if inMsg.Message.IsCommand() {
		return processChatCommand(ctx, inMsg)
	}
	return chatResponses(ctx, inMsg)
}

func processChatCommand(ctx context.Context, inMsg *tgbotapi.Update) error {
	cmd := inMsg.Message.Command()
	switch cmd {
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
		gcClaim(ctx, inMsg)
	case "reject":
		gcReject(ctx, inMsg)
	case "acknowledge":
		gcAcknowledge(ctx, inMsg)
	default:
		res := newGroupResponse(inMsg.Message.Chat.ID)
		from := inMsg.Message.From

		gid, err := model.TelegramID(from.ID).Gid(ctx)
		if err != nil {
			sendError(&res, err)
			return err
		}

		res.Text, _ = templates.ExecuteLang("default", from.LanguageCode, commands)
		log.Debugw("unknown command in chat", "chatID", inMsg.Message.Chat.ID, "GID", gid, "cmd", cmd)
		sendQueue <- res
	}
	return nil
}

func chatResponses(ctx context.Context, inMsg *tgbotapi.Update) error {
	teamID, _, err := model.ChatToTeam(ctx, inMsg.Message.Chat.ID)
	if err != nil {
		return nil // Not a linked chat
	}

	// 1. Bot removed from chat
	if inMsg.Message.LeftChatMember != nil && inMsg.Message.LeftChatMember.ID == bot.Self.ID {
		log.Infow("bot removed from chat, unlinking team", "teamID", teamID)
		return teamID.UnlinkFromTelegramChat(ctx)
	}

	// 2. New members added
	if inMsg.Message.NewChatMembers != nil {
		for _, member := range inMsg.Message.NewChatMembers {
			tgid := model.TelegramID(member.ID)
			gid, err := tgid.Gid(ctx)
			if err != nil {
				continue
			}
			_ = tgid.SetName(ctx, member.UserName)
			if err := teamID.AddAgent(ctx, gid); err != nil {
				log.Errorw("failed to add agent to team on join", "error", err, "teamID", teamID, "GID", gid)
			}
		}
	}

	// 3. Members leaving
	if inMsg.Message.LeftChatMember != nil {
		left := inMsg.Message.LeftChatMember
		tgid := model.TelegramID(left.ID)
		if gid, err := tgid.Gid(ctx); err == nil {
			if err := teamID.RemoveAgent(ctx, gid); err != nil {
				log.Errorw("failed to remove agent from team on leave", "error", err, "teamID", teamID, "GID", gid)
			}
		}
	}
	return nil
}

func addToChat(ctx context.Context, g messaging.GoogleID, t messaging.TeamID) error {
	gid, teamID := model.GoogleID(g), model.TeamID(t)
	chatID, err := teamID.TelegramChat(ctx)
	if err != nil || chatID == 0 {
		return err
	}

	tgid, err := gid.TelegramID(ctx)
	if err != nil || tgid == 0 {
		// If we don't know the TGID, we can't invite them, tell the chat
		text, _ := templates.ExecuteLang("agentUnknown", "en", gid)
		msg := tgbotapi.NewMessage(chatID, text)
		sendQueue <- msg
		return nil
	}

	if model.IsChatMember(ctx, tgid, model.TelegramID(chatID)) {
		return nil
	}

	// Get names for template
	name, _ := gid.IngressName(ctx)
	if tmp, _ := gid.TelegramName(ctx); tmp != "" {
		name = "@" + tmp
	}
	teamname, _ := teamID.Name(ctx)

	_ = model.AddToChatMemberList(ctx, tgid, model.TelegramID(chatID))

	d := struct {
		Name     string
		TeamName string
		TeamID   model.TeamID
		SentLink bool
	}{
		Name:     name,
		TeamName: teamname,
		TeamID:   teamID,
		SentLink: true,
	}

	if err := sendInviteLink(ctx, tgid, chatID, teamname); err != nil {
		d.SentLink = false
	}

	text, _ := templates.ExecuteLang("joinedTeam", "en", d)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	sendQueue <- msg
	return nil
}

func sendInviteLink(ctx context.Context, tgid model.TelegramID, chatID int64, team string) error {
	// The struct uses BaseChat, but the helper NewCreateChatInviteLink is safer
	config := tgbotapi.CreateChatInviteLinkConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: chatID,
		},
		MemberLimit: 1,
	}

	res, err := bot.Request(config)
	if err != nil {
		return err
	}

	var r struct {
		Link string `json:"invite_link"`
	}
	// Note: We are unmarshalling the Result field from the generic API response
	if err := json.Unmarshal(res.Result, &r); err != nil {
		return err
	}

	d := struct {
		TeamName string
		Link     string
	}{
		TeamName: team,
		Link:     r.Link,
	}

	message, _ := templates.ExecuteLang("invitedToTeam", "en", d)
	msg := tgbotapi.NewMessage(int64(tgid), message)
	msg.ParseMode = tgbotapi.ModeHTML
	sendQueue <- msg
	return nil
}

func removeFromChat(ctx context.Context, g messaging.GoogleID, t messaging.TeamID) error {
	gid, teamID := model.GoogleID(g), model.TeamID(t)
	chatID, err := teamID.TelegramChat(ctx)
	if err != nil || chatID == 0 {
		return err
	}

	tgid, err := gid.TelegramID(ctx)
	if err != nil || tgid == 0 {
		return nil
	}

	bcmc := tgbotapi.BanChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: chatID,
			UserID: int64(tgid),
		},
		UntilDate: time.Now().Add(30 * time.Second).Unix(),
	}

	if _, err = bot.Request(bcmc); err != nil {
		errStr := err.Error()
		if errStr == "Bad Request: USER_ID_INVALID" {
			_ = gid.RemoveTelegramID(ctx)
		} else if errStr != "Bad Request: USER_NOT_PARTICIPANT" {
			log.Errorw("kick failed", "error", err, "tgid", tgid, "chatID", chatID)
		}
	}

	_ = model.RemoveFromChatMemberList(ctx, tgid, model.TelegramID(chatID))
	return nil
}

func liveLocationUpdate(ctx context.Context, inMsg *tgbotapi.Update) error {
	tgid := model.TelegramID(inMsg.EditedMessage.From.ID)
	gid, verified, err := tgid.GidV(ctx)
	if err != nil {
		return err
	}
	if !verified || gid == "" {
		return nil
	}

	lat := strconv.FormatFloat(inMsg.EditedMessage.Location.Latitude, 'f', -1, 64)
	lon := strconv.FormatFloat(inMsg.EditedMessage.Location.Longitude, 'f', -1, 64)
	return gid.SetLocation(ctx, lat, lon)
}
