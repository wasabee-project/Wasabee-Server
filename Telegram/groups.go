package wasabeetelegram

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/wasabee-project/Wasabee-Server"
)

func processChatMessage(inMsg *tgbotapi.Update) error {
	if inMsg.Message.IsCommand() {
		return processChatCommand(inMsg)
	}
	return chatResponses(inMsg)
}

func processChatCommand(inMsg *tgbotapi.Update) error {
	tgid := wasabee.TelegramID(inMsg.Message.From.ID)
	gid, _, err := tgid.GidV()
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}

	msg := tgbotapi.NewMessage(inMsg.Message.Chat.ID, "")
	defaultReply, err := templateExecute("default", inMsg.Message.From.LanguageCode, nil)
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	msg.ParseMode = "HTML"
	msg.Text = defaultReply

	switch inMsg.Message.Command() {
	case "link":
		tokens := strings.Split(inMsg.Message.Text, " ")
		if len(tokens) == 2 {
			team := wasabee.TeamID(strings.TrimSpace(tokens[1]))
			wasabee.Log.Debugw("linking team and chat", "chatID", inMsg.Message.Chat.ID, "GID", gid, "resource", team)
			if err := team.LinkToTelegramChat(inMsg.Message.Chat.ID, gid); err != nil {
				wasabee.Log.Error(err)
				msg.Text = err.Error()
				if _, err := bot.Send(msg); err != nil {
					wasabee.Log.Error(err)
					return err
				}
				return err
			}
		} else {
			msg.Text = "specify a single teamID"
			if _, err := bot.Send(msg); err != nil {
				wasabee.Log.Error(err)
				return err
			}
			return nil
		}
		msg.Text = "Successfully linked"
		if _, err := bot.Send(msg); err != nil {
			wasabee.Log.Error(err)
			return err
		}
	case "status":
		msg.ParseMode = "HTML"
		teamID, err := wasabee.ChatToTeam(inMsg.Message.Chat.ID)
		if err != nil {
			wasabee.Log.Error(err)
			msg.Text = err.Error()
			if _, err := bot.Send(msg); err != nil {
				wasabee.Log.Error(err)
				return err
			}
			return err
		}
		name, _ := teamID.Name()
		msg.Text = fmt.Sprintf("Linked to team: <b>%s</b> (%s)", name, teamID.String())
		if _, err := bot.Send(msg); err != nil {
			wasabee.Log.Error(err)
			return err
		}
	case "assignments":
		msg.ParseMode = "HTML"
		teamID, err := wasabee.ChatToTeam(inMsg.Message.Chat.ID)
		if err != nil {
			wasabee.Log.Error(err)
			if _, err := bot.Send(msg); err != nil {
				wasabee.Log.Error(err)
				return err
			}
			return err
		}
		ops, err := teamID.Operations()
		if err != nil {
			wasabee.Log.Error(err)
			return err
		}
		for _, p := range ops {
			var o wasabee.Operation
			o.ID = p.OpID
			err := o.Populate(gid)
			if err != nil {
				wasabee.Log.Error(err)
				continue
			}
			var b bytes.Buffer
			name, _ := teamID.Name()
			b.WriteString(fmt.Sprintf("<b>Operation: %s</b> (team: %s)\n", o.Name, name))
			b.WriteString("<b>Order / Portal / Action / Agent / State</b>\n")
			for _, m := range o.Markers {
				if m.State != "pending" && m.AssignedTo != "" {
					p, _ := o.PortalDetails(m.PortalID, gid)
					a, _ := m.AssignedTo.IngressNameTeam(teamID)
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
						m.Order, stateIndicatorStart, p.Lat, p.Lon, p.Name, wasabee.NewMarkerType(m.Type), a, m.State, stateIndicatorEnd))
				}
			}
			msg.Text = b.String()
			if _, err := bot.Send(msg); err != nil {
				wasabee.Log.Error(err)
				msg.Text = err.Error()
				bot.Send(msg)
				continue
			}
		}
	case "unassigned":
		msg.ParseMode = "HTML"
		teamID, err := wasabee.ChatToTeam(inMsg.Message.Chat.ID)
		if err != nil {
			wasabee.Log.Error(err)
			if _, err := bot.Send(msg); err != nil {
				wasabee.Log.Error(err)
				return err
			}
			return err
		}
		ops, err := teamID.Operations()
		if err != nil {
			wasabee.Log.Error(err)
			return err
		}
		for _, p := range ops {
			var o wasabee.Operation
			o.ID = p.OpID
			err := o.Populate(gid)
			if err != nil {
				wasabee.Log.Error(err)
				continue
			}
			var b bytes.Buffer
			name, _ := teamID.Name()
			b.WriteString(fmt.Sprintf("<b>Operation: %s</b> (team: %s)\n", o.Name, name))
			b.WriteString("<b>Order / Portal / Action</b>\n")
			for _, m := range o.Markers {
				if m.State == "pending" {
					p, _ := o.PortalDetails(m.PortalID, gid)
					b.WriteString(fmt.Sprintf("<b>%d</b> / <a href=\"http://maps.google.com/?q=%s,%s\">%s</a> / %s\n", m.Order, p.Lat, p.Lon, p.Name, wasabee.NewMarkerType(m.Type)))
				}
			}
			msg.Text = b.String()
			if _, err := bot.Send(msg); err != nil {
				wasabee.Log.Error(err)
				continue
			}
		}
	default:
		wasabee.Log.Debugw("unknown command in chat", "chatID", inMsg.Message.Chat.ID, "GID", gid, "cmd", inMsg.Message.Command())
	}
	return nil
}

func chatResponses(inMsg *tgbotapi.Update) error {
	// wasabee.Log.Debugw("message in chat", "chatID", inMsg.Message.Chat.ID, "GID", gid)
	if inMsg.Message.LeftChatMember != nil && inMsg.Message.LeftChatMember.ID == bot.Self.ID {
		teamID, err := wasabee.ChatToTeam(inMsg.Message.Chat.ID)
		if err != nil {
			wasabee.Log.Error(err)
			return err
		}
		if err := teamID.UnlinkFromTelegramChat(inMsg.Message.Chat.ID); err != nil {
			wasabee.Log.Error(err)
			return err
		}
	}
	// we can log being added to a chat using inMsg.Message.NewChatMembers
	return nil
}

func liveLocationUpdate(inMsg *tgbotapi.Update) error {
	tgid := wasabee.TelegramID(inMsg.EditedMessage.From.ID)
	gid, verified, err := tgid.GidV()
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	if !verified || gid == "" {
		// wasabee.Log.Debugw("user not initialized/verified, ignoring location", "GID", gid, "tgid", tgid)
		return nil
	}
	// wasabee.Log.Debugw("live location inMsg", "GID", gid, "message", "live location update")

	_ = gid.AgentLocation(
		strconv.FormatFloat(inMsg.EditedMessage.Location.Latitude, 'f', -1, 64),
		strconv.FormatFloat(inMsg.EditedMessage.Location.Longitude, 'f', -1, 64),
	)
	return nil
}

// SendToTeamChannel sends a message to chat linked to a team
func SendToTeamChannel(teamID wasabee.TeamID, gid wasabee.GoogleID, message string) error {
	chatID, err := teamID.TelegramChat()
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}

	// XXX make sure sender is on the team

	msg := tgbotapi.NewMessage(chatID, "")

	msg.Text = message
	msg.ParseMode = "HTML"
	if _, err := bot.Send(msg); err != nil {
		wasabee.Log.Error(err)
		return err
	}

	return nil
}
