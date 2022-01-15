package wtg

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/templates"
)

func processChatMessage(inMsg *tgbotapi.Update) error {
	if !model.IsChatMember(model.TelegramID(inMsg.Message.From.ID), model.TelegramID(inMsg.Message.Chat.ID)) {
		log.Debugw("adding agent to chat list", "agent", inMsg.Message.From.ID, "chat", inMsg.Message.Chat.ID)
		if err := model.AddToChatMemberList(model.TelegramID(inMsg.Message.From.ID), model.TelegramID(inMsg.Message.Chat.ID)); err != nil {
			log.Debug(err)
			text, _ := templates.ExecuteLang("agentUnknown", inMsg.Message.From.LanguageCode, inMsg.Message.From.UserName)
			msg := tgbotapi.NewMessage(inMsg.Message.Chat.ID, text)
			sendQueue <- msg
		}
	}

	if inMsg.Message.IsCommand() {
		return processChatCommand(inMsg)
	}
	return chatResponses(inMsg)
}

func processChatCommand(inMsg *tgbotapi.Update) error {
	switch inMsg.Message.Command() {
	case "unlink":
		gcUnlink(inMsg)
	case "link":
		gcLink(inMsg)
	case "status":
		gcStatus(inMsg)
	case "assignments", "assigned":
		gcAssigned(inMsg)
	case "unassigned":
		gcUnassigned(inMsg)
	case "claim":
		gcClaim(inMsg)
	case "Reject":
		gcReject(inMsg)
	case "acknowledge":
		gcAcknowledge(inMsg)
	default:
		msg := tgbotapi.NewMessage(inMsg.Message.Chat.ID, "")
		msg.ParseMode = "HTML"
		msg.DisableWebPagePreview = true

		gid, err := model.TelegramID(inMsg.Message.From.ID).Gid()
		if err != nil {
			log.Error(err)
			msg.Text = err.Error()
			sendQueue <- msg
			return err
		}
		msg.Text, err = templates.ExecuteLang("default", inMsg.Message.From.LanguageCode, commands)
		if err != nil {
			log.Error(err)
			msg.Text = err.Error()
		}
		log.Debugw("unknown command in chat", "chatID", inMsg.Message.Chat.ID, "GID", gid, "cmd", inMsg.Message.Command())
		sendQueue <- msg
	}
	return nil
}

func chatResponses(inMsg *tgbotapi.Update) error {
	teamID, opID, err := model.ChatToTeam(inMsg.Message.Chat.ID)
	if err != nil {
		// no need to log these, just non-linked chats
		// log.Debugw("unknown chat", "err", err.Error(), "chatID", inMsg.Message.Chat.ID)
		return nil
	}

	// we see messages if the bot is admin
	// log.Debugw("message in chat", "chatID", inMsg.Message.Chat.ID, "team", teamID, "op", opID)
	// if we see a message in a chat from someone not on the team, add them?

	// if the bot is removed from the chat, unlink the team from the chat
	if inMsg.Message.LeftChatMember != nil && inMsg.Message.LeftChatMember.ID == bot.Self.ID {
		if err := teamID.UnlinkFromTelegramChat(); err != nil {
			log.Error(err)
			return err
		}
	}

	// when new people are added to the chat, attempt to add them to the team
	if inMsg.Message.NewChatMembers != nil {
		for _, new := range inMsg.Message.NewChatMembers {
			log.Debugw("new chat member", "tgid", new.ID, "tg", new.UserName)
			tgid := model.TelegramID(new.ID)
			gid, err := tgid.Gid()
			if err != nil {
				continue
			}
			_ = tgid.SetName(new.UserName)
			if err = teamID.AddAgent(gid); err != nil {
				log.Errorw(err.Error(), "tgid", new.ID, "tg", new.UserName, "resource", teamID, "GID", gid, "opID", opID)
			}
		}
	}

	if inMsg.Message.LeftChatMember != nil {
		left := inMsg.Message.LeftChatMember
		log.Debugw("chat member left", "tgid", left.ID, "tg", left.UserName)
		tgid := model.TelegramID(left.ID)
		gid, err := tgid.Gid()
		if err != nil {
			log.Debugw(err.Error(), "tgid", left.ID, "tg", left.UserName, "resource", teamID, "opID", opID)
		} else {
			if err := teamID.RemoveAgent(gid); err != nil {
				log.Errorw(err.Error(), "tgid", left.ID, "tg", left.UserName, "resource", teamID, "GID", gid, "opID", opID)
			}
		}
	}
	return nil
}

func liveLocationUpdate(inMsg *tgbotapi.Update) error {
	tgid := model.TelegramID(inMsg.EditedMessage.From.ID)
	gid, verified, err := tgid.GidV()
	if err != nil {
		log.Error(err)
		return err
	}
	if !verified || gid == "" {
		return nil
	}

	lat := strconv.FormatFloat(inMsg.EditedMessage.Location.Latitude, 'f', -1, 64)
	lon := strconv.FormatFloat(inMsg.EditedMessage.Location.Longitude, 'f', -1, 64)
	_ = gid.SetLocation(lat, lon)
	return nil
}

// sendToTeamChannel sends a message to the primary (not linked to an op) chat linked to a team
// unused
func sendToTeamChannel(teamID model.TeamID, gid model.GoogleID, message string) error {
	chatID, err := teamID.TelegramChat()
	if err != nil {
		log.Error(err)
		return err
	}

	if inteam, _ := gid.AgentInTeam(teamID); !inteam {
		err := fmt.Errorf("attempt to send to team without being a member")
		log.Errorw(err.Error(), "gid", gid, "teamID", teamID, "message", message)
		return err
	}

	msg := tgbotapi.NewMessage(chatID, message)
	msg.ParseMode = "HTML"
	msg.DisableWebPagePreview = true

	sendQueue <- msg
	return nil
}

func addToChat(g messaging.GoogleID, t messaging.TeamID) error {
	lang := "en"
	gid := model.GoogleID(g)
	teamID := model.TeamID(t)

	chatID, err := teamID.TelegramChat()
	if err != nil {
		log.Error(err)
		return err
	}

	if chatID == 0 {
		return nil
	}

	cic := tgbotapi.ChatInfoConfig{}
	cic.ChatID = chatID
	chat, err := bot.GetChat(cic)
	if err != nil {
		log.Errorw(err.Error(), "chatID", chatID, "GID", gid)
		if err.Error() == "Bad Request: chat not found" {
			_ = teamID.UnlinkFromTelegramChat()
		}
		return err
	}

	tgid, err := gid.TelegramID()
	if err != nil {
		log.Error(err)
		return err
	}
	if tgid == 0 {
		text, _ := templates.ExecuteLang("agentUnknown", lang, gid)
		msg := tgbotapi.NewMessage(chat.ID, text)
		sendQueue <- msg
		return nil
	}

	if model.IsChatMember(tgid, model.TelegramID(chatID)) {
		log.Debug("not adding agent to chat since already a member")
		return nil
	}

	name, _ := gid.IngressName()
	if tmp, _ := gid.TelegramName(); tmp != "" {
		name = fmt.Sprint("@", tmp)
	}

	teamname, err := teamID.Name()
	if err != nil {
		log.Error(err)
		teamname = string(teamID)
	}

	if err := model.AddToChatMemberList(tgid, model.TelegramID(chatID)); err != nil {
		log.Error(err)
	}

	type data struct {
		Name     string
		TeamName string
		TeamID   model.TeamID
		SentLink bool
	}
	d := data{
		Name:     name,
		TeamName: teamname,
		TeamID:   teamID,
		SentLink: true,
	}

	if err := sendInviteLink(tgid, chatID, teamname); err != nil {
		d.SentLink = false
	}

	text, _ := templates.ExecuteLang("joinedTeam", lang, d)
	sendQueue <- tgbotapi.NewMessage(chat.ID, text)

	return nil
}

func sendInviteLink(tgid model.TelegramID, chatID int64, team string) error {
	ccilc := tgbotapi.CreateChatInviteLinkConfig{}
	ccilc.ChatID = chatID
	ccilc.MemberLimit = 1
	res, err := bot.Request(ccilc)
	if err != nil {
		if err.Error() != "Bad Request: not enough rights to manage chat invite link" {
			log.Error(err)
		}
		return err
	}

	var r struct {
		Link    string `json:"invite_link"`
		Revoked bool   `json:"is_revoked"`
	}
	if err := json.Unmarshal(res.Result, &r); err != nil {
		log.Error(err)
		return err
	}
	if r.Revoked { // has this ever been triggered?
		err := fmt.Errorf("join linked already revoked")
		log.Error(err)
		return err
	}

	type data struct {
		TeamName string
		Link     string
	}
	message, _ := templates.ExecuteLang("invitedToTeam", "en", data{TeamName: team, Link: r.Link})
	msg := tgbotapi.NewMessage(int64(tgid), message)
	msg.ParseMode = "HTML"
	sendQueue <- msg
	return nil
}

func removeFromChat(g messaging.GoogleID, t messaging.TeamID) error {
	gid := model.GoogleID(g)
	teamID := model.TeamID(t)

	chatID, err := teamID.TelegramChat()
	if err != nil {
		log.Error(err)
		return err
	}
	if chatID == 0 {
		return nil
	}

	tgid, err := gid.TelegramID()
	if err != nil {
		log.Error(err)
		return err
	}
	if tgid == 0 {
		return nil
	}

	cic := tgbotapi.ChatInfoConfig{}
	cic.ChatID = chatID

	chat, err := bot.GetChat(cic)
	if err != nil {
		log.Errorw(err.Error(), "chatID", chatID, "GID", gid)
		return err
	}
	name, _ := gid.IngressName()
	if tmp, _ := gid.TelegramName(); tmp != "" {
		name = fmt.Sprint("@", tmp)
	}

	// determine if bot is admin, don't bother with this if not
	// bot.GetChatAdministrator doesn't list admin bots...
	cmc := tgbotapi.GetChatMemberConfig{}
	cmc.ChatID = chatID
	cmc.UserID = bot.Self.ID
	cm, err := bot.GetChatMember(cmc)
	if err != nil {
		log.Error(err)
		return err
	}

	type data struct {
		Agent  string
		TeamID model.TeamID
		Admin  bool
	}
	message, _ := templates.ExecuteLang("leftTeam", "en", data{Agent: name, TeamID: teamID, Admin: cm.IsAdministrator()})
	msg := tgbotapi.NewMessage(chat.ID, message)
	msg.ParseMode = "HTML"
	sendQueue <- msg

	_ = model.RemoveFromChatMemberList(tgid, model.TelegramID(chatID))

	if !cm.IsAdministrator() {
		log.Debug("I am not admin... trying anyways")
	} else {
		log.Debug("I AM admin!")
	}

	bcmc := tgbotapi.BanChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: chatID,
			UserID: int64(tgid),
		},
		UntilDate:      time.Now().Add(30 * time.Second).Unix(),
		RevokeMessages: false,
	}
	if _, err = bot.Request(bcmc); err != nil {
		errstr := err.Error()
		switch errstr {
		case "Bad Request: USER_ID_INVALID":
			log.Infow("invalid telegram ID, clearing from agent", "gid", gid, "tgid", tgid)
			if err := gid.RemoveTelegramID(); err != nil {
				log.Error(err)
				msg := tgbotapi.NewMessage(chat.ID, err.Error())
				sendQueue <- msg
			}
		case "Bad Request: USER_NOT_PARTICIPANT":
			// nothing
		default:
			log.Error(err)
			msg := tgbotapi.NewMessage(chat.ID, err.Error())
			sendQueue <- msg
		}
	}

	return nil
}
