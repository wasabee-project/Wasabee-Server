package wtg

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/templates"
)

func processChatMessage(inMsg *tgbotapi.Update) error {
	if inMsg.Message.IsCommand() {
		return processChatCommand(inMsg)
	}
	return chatResponses(inMsg)
}

func processChatCommand(inMsg *tgbotapi.Update) error {
	gid, err := model.TelegramID(inMsg.Message.From.ID).Gid()
	if err != nil {
		log.Error(err)
		return err
	}

	defaultReply, err := templates.ExecuteLang("default", inMsg.Message.From.LanguageCode, nil)
	if err != nil {
		log.Error(err)
		return err
	}
	msg := tgbotapi.NewMessage(inMsg.Message.Chat.ID, defaultReply)
	msg.ParseMode = "HTML"
	msg.DisableWebPagePreview = true

	switch inMsg.Message.Command() {
	case "unlink":
		teamID, _, _ := model.ChatToTeam(inMsg.Message.Chat.ID)
		log.Debugw("unlinking team from chat", "chatID", inMsg.Message.Chat.ID, "GID", gid, "resource", teamID)

		owns, err := gid.OwnsTeam(teamID)
		if err != nil {
			log.Error(err)
			return err
		}
		if !owns {
			err = fmt.Errorf("only team owner can unlink the team")
			log.Error(err)
			return err
		}

		if err := teamID.UnlinkFromTelegramChat(model.TelegramID(inMsg.Message.Chat.ID)); err != nil {
			log.Error(err)
			msg.Text = err.Error()
			sendQueue <- msg
			return err
		}

		msg.Text = "Successfully unlinked"
		sendQueue <- msg
	case "link":
		tokens := strings.Split(inMsg.Message.Text, " ")
		if len(tokens) > 1 {
			team := model.TeamID(strings.TrimSpace(tokens[1]))
			var opID model.OperationID
			if len(tokens) == 3 {
				opID = model.OperationID(strings.TrimSpace(tokens[2]))
			}
			log.Debugw("linking team and chat", "chatID", inMsg.Message.Chat.ID, "GID", gid, "resource", team, "opID", opID)

			owns, err := gid.OwnsTeam(team)
			if err != nil {
				log.Error(err)
				return err
			}
			if !owns {
				err = fmt.Errorf("only team owner can set telegram link")
				log.Error(err)
				msg.Text = err.Error()
				sendQueue <- msg
				return err
			}

			if err := team.LinkToTelegramChat(model.TelegramID(inMsg.Message.Chat.ID), opID); err != nil {
				log.Error(err)
				msg.Text = err.Error()
				sendQueue <- msg
				return err
			}
		} else {
			msg.Text = "specify a single teamID"
			sendQueue <- msg
			return nil
		}
		msg.Text = "Successfully linked"
		sendQueue <- msg
	case "status":
		teamID, opID, err := model.ChatToTeam(inMsg.Message.Chat.ID)
		if err != nil {
			// log.Debug(err) // not linked is not an error
			msg.Text = err.Error()
			sendQueue <- msg
			return nil
		}
		name, _ := teamID.Name()

		msg.Text = fmt.Sprintf("Linked to team: <b>%s</b> (%s)", name, teamID.String())

		if opID != "" {
			opstat, err := opID.Stat()
			if err != nil {
				log.Error(err)
			} else {
				msg.Text = fmt.Sprintf("%s \nLinked to Operation <b>%s</b> (%s)", msg.Text, opstat.Name, opID)
			}
		}
		sendQueue <- msg
	case "assignments":
		var filterGid model.GoogleID
		msg.ParseMode = "HTML"
		msg.DisableWebPagePreview = true
		tokens := strings.Split(inMsg.Message.Text, " ")
		if len(tokens) > 1 {
			filterGid, err = model.SearchAgentName(strings.TrimSpace(tokens[1]))
			if err != nil {
				log.Error(err)
				filterGid = "0"
			}
		} else {
			filterGid = ""
		}
		teamID, opID, err := model.ChatToTeam(inMsg.Message.Chat.ID)
		if err != nil {
			log.Error(err)
			msg.Text = err.Error()
			sendQueue <- msg
			return err
		}
		if opID == "" {
			err := fmt.Errorf("team must be linked to operation to view assignments")
			msg.Text = err.Error()
			sendQueue <- msg
			return err
		}
		o := model.Operation{}
		o.ID = opID
		if err := o.Populate(gid); err != nil {
			log.Error(err)
			msg.Text = err.Error()
			sendQueue <- msg
			return err
		}
		var b bytes.Buffer
		name, _ := teamID.Name()
		b.WriteString(fmt.Sprintf("<b>Operation: %s</b> (team: %s)\n", o.Name, name))
		b.WriteString("<b>Order / Portal / Action / Agent / State</b>\n")
		for _, m := range o.Markers {
			// if the caller requested the results to be filtered...
			if filterGid != "" && m.IsAssignedTo(filterGid) {
				continue
			}
			if m.State != "pending" {
				p, _ := o.PortalDetails(m.PortalID, gid)
				a, _ := m.AssignedTo.IngressName()
				tg, _ := m.AssignedTo.TelegramName()
				if tg != "" {
					a = fmt.Sprintf("@%s", tg)
				}
				stateIndicatorStart := ""
				stateIndicatorEnd := ""
				if m.State == "completed" {
					stateIndicatorStart = "<strike>"
					stateIndicatorEnd = "</strike>"
				}
				b.WriteString(fmt.Sprintf("%d / %s<a href=\"http://maps.google.com/?q=%s,%s\">%s</a> / %s / %s / %s%s\n",
					m.Order, stateIndicatorStart, p.Lat, p.Lon, p.Name, model.NewMarkerType(m.Type), a, m.State, stateIndicatorEnd))
			}
			msg.Text = b.String()
		}
		sendQueue <- msg
	case "unassigned":
		teamID, opID, err := model.ChatToTeam(inMsg.Message.Chat.ID)
		if err != nil {
			log.Error(err)
			msg.Text = err.Error()
			sendQueue <- msg
			return err
		}
		if opID == "" {
			err := fmt.Errorf("team must be linked to operation to view assignments")
			msg.Text = err.Error()
			sendQueue <- msg
			return err
		}
		o := model.Operation{}
		o.ID = opID
		if err := o.Populate(gid); err != nil {
			log.Error(err)
			msg.Text = err.Error()
			sendQueue <- msg
			return err
		}
		var b bytes.Buffer
		name, _ := teamID.Name()
		b.WriteString(fmt.Sprintf("<b>Operation: %s</b> (team: %s)\n", o.Name, name))
		b.WriteString("<b>Order / Portal / Action</b>\n")
		for _, m := range o.Markers {
			if m.State == "pending" {
				p, _ := o.PortalDetails(m.PortalID, gid)
				b.WriteString(fmt.Sprintf("<b>%d</b> / <a href=\"http://maps.google.com/?q=%s,%s\">%s</a> / %s\n", m.Order, p.Lat, p.Lon, p.Name, model.NewMarkerType(m.Type)))
			}
			msg.Text = b.String()
		}
		sendQueue <- msg
	case "claim":
		_, opID, err := model.ChatToTeam(inMsg.Message.Chat.ID)
		if err != nil {
			log.Error(err)
			msg.Text = err.Error()
			sendQueue <- msg
			return err
		}
		if opID == "" {
			err := fmt.Errorf("team must be linked to operation to view assignments")
			msg.Text = err.Error()
			sendQueue <- msg
			return err
		}
		o := model.Operation{}
		o.ID = opID
		if err := o.Populate(gid); err != nil {
			log.Error(err)
			msg.Text = err.Error()
			sendQueue <- msg
			return err
		}
		tokens := strings.Split(inMsg.Message.Text, " ")
		step, err := strconv.Atoi(strings.TrimSpace(tokens[1]))
		if err != nil {
			log.Error(err)
			msg.Text = err.Error()
			sendQueue <- msg
			return err
		}
		task, err := o.GetTaskByStepNumber(int16(step))
		if err != nil {
			log.Error(err)
			msg.Text = err.Error()
			sendQueue <- msg
			return err
		}
		if err := task.Claim(gid); err != nil {
			log.Error(err)
			msg.Text = err.Error()
			sendQueue <- msg
			return err
		}
	default:
		log.Debugw("unknown command in chat", "chatID", inMsg.Message.Chat.ID, "GID", gid, "cmd", inMsg.Message.Command())
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
		if err := teamID.UnlinkFromTelegramChat(model.TelegramID(inMsg.Message.Chat.ID)); err != nil {
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

// SendToTeamChannel sends a message to chat linked to a team
func SendToTeamChannel(teamID model.TeamID, gid model.GoogleID, message string) error {
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
	gid := model.GoogleID(g)
	teamID := model.TeamID(t)
	// log.Debugw("AddToChat called", "GID", gid, "resource", teamID)

	chatID, err := teamID.TelegramChat()
	if err != nil {
		log.Error(err)
		return err
	}
	if chatID == 0 {
		// log.Debug("no chat linked to team")
		return nil
	}

	// can't clean this up since cic.ChatID is included type
	cic := tgbotapi.ChatInfoConfig{}
	cic.ChatID = chatID
	chat, err := bot.GetChat(cic)
	if err != nil {
		log.Errorw(err.Error(), "chatID", chatID, "GID", gid)
		if err.Error() == "Bad Request: chat not found" {
			_ = teamID.UnlinkFromTelegramChat(model.TelegramID(chatID))
		}
		return err
	}

	name, _ := gid.IngressName()
	if tmp, _ := gid.TelegramName(); tmp != "" {
		name = fmt.Sprint("@", tmp)
	}

	text := fmt.Sprintf("%s joined the linked team (%s): Please add them to this chat", name, teamID)
	msg := tgbotapi.NewMessage(chat.ID, text)
	sendQueue <- msg

	// XXX create a join link for this agent
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
	// log.Debugw("RemoveFromChat called", "GID", gid, "teamID", teamID, "chatID", chatID, "tgid", tgid)

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

	text := fmt.Sprintf("%s left the linked team (%s). Attempting to remove them from this chat", name, teamID)
	msg := tgbotapi.NewMessage(chat.ID, text)
	sendQueue <- msg

	// XXX determine if bot is admin, don't bother with this if not
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
			gid.RemoveTelegramID()
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
