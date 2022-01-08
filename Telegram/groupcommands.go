package wtg

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/templates"
)

func gcUnlink(inMsg *tgbotapi.Update) {
	msg := tgbotapi.NewMessage(inMsg.Message.Chat.ID, "")
	msg.ParseMode = "HTML"
	msg.DisableWebPagePreview = true

	gid, err := model.TelegramID(inMsg.Message.From.ID).Gid()
	if err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}

	teamID, _, _ := model.ChatToTeam(inMsg.Message.Chat.ID)
	// log.Debugw("unlinking team from chat", "chatID", inMsg.Message.Chat.ID, "GID", gid, "resource", teamID)

	owns, err := gid.OwnsTeam(teamID)
	if err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}

	if !owns {
		err = fmt.Errorf("only team owner can unlink the team")
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}

	if err := teamID.UnlinkFromTelegramChat(); err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}

	msg.Text, err = templates.ExecuteLang("Unlinked", inMsg.Message.From.LanguageCode, nil)
	if err != nil {
		log.Error(err)
		// do something?
	}
	sendQueue <- msg
}

func gcLink(inMsg *tgbotapi.Update) {
	msg := tgbotapi.NewMessage(inMsg.Message.Chat.ID, "")
	msg.ParseMode = "HTML"
	msg.DisableWebPagePreview = true

	gid, err := model.TelegramID(inMsg.Message.From.ID).Gid()
	if err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}

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
			msg.Text = err.Error()
			sendQueue <- msg
			return
		}
		if !owns {
			err = fmt.Errorf("only team owner can set telegram link")
			log.Error(err)
			msg.Text = err.Error()
			sendQueue <- msg
			return
		}

		if err := team.LinkToTelegramChat(model.TelegramID(inMsg.Message.Chat.ID), opID); err != nil {
			log.Error(err)
			msg.Text = err.Error()
			sendQueue <- msg
			return
		}
	} else {
		msg.Text, _ = templates.ExecuteLang("SingleTeam", inMsg.Message.From.LanguageCode, nil)
		sendQueue <- msg
		return
	}
	msg.Text, _ = templates.ExecuteLang("Linked", inMsg.Message.From.LanguageCode, nil)
	sendQueue <- msg
}

func gcStatus(inMsg *tgbotapi.Update) {
	msg := tgbotapi.NewMessage(inMsg.Message.Chat.ID, "")
	msg.ParseMode = "HTML"
	msg.DisableWebPagePreview = true

	teamID, opID, err := model.ChatToTeam(inMsg.Message.Chat.ID)
	if err != nil {
		// log.Debug(err) // not linked is not an error
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}
	name, _ := teamID.Name()

	type data struct {
		TeamName string
		TeamID   model.TeamID
		OPStat   *model.OpStat
	}
	d := data{
		TeamName: name,
		TeamID:   teamID,
	}

	if opID != "" {
		d.OPStat, err = opID.Stat()
		if err != nil {
			log.Error(err)
		}
	}

	msg.Text, err = templates.ExecuteLang("ChatLinkStatus", inMsg.Message.From.LanguageCode, d)
	if err != nil {
		log.Error(err)
	}
	sendQueue <- msg
}

func gcAssigned(inMsg *tgbotapi.Update) {
	msg := tgbotapi.NewMessage(inMsg.Message.Chat.ID, "")
	msg.ParseMode = "HTML"
	msg.DisableWebPagePreview = true

	gid, err := model.TelegramID(inMsg.Message.From.ID).Gid()
	if err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}

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
		return
	}
	if opID == "" {
		err := fmt.Errorf("team must be linked to operation to view assignments")
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}
	o := model.Operation{}
	o.ID = opID
	if err := o.Populate(gid); err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}
	var b bytes.Buffer
	name, _ := teamID.Name()
	b.WriteString(fmt.Sprintf("<b>Operation: %s</b> (team: %s)\n", o.Name, name))
	b.WriteString("<b>Order / Portal / Action / Agent / State</b>\n")
	sort.Slice(o.Markers, func(i, j int) bool { return o.Markers[i].Order < o.Markers[j].Order })
	for _, m := range o.Markers {
		// if the caller requested the results to be filtered...
		if filterGid != "" && !m.IsAssignedTo(filterGid) {
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
}

func gcUnassigned(inMsg *tgbotapi.Update) {
	msg := tgbotapi.NewMessage(inMsg.Message.Chat.ID, "")
	msg.ParseMode = "HTML"
	msg.DisableWebPagePreview = true

	gid, err := model.TelegramID(inMsg.Message.From.ID).Gid()
	if err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}

	teamID, opID, err := model.ChatToTeam(inMsg.Message.Chat.ID)
	if err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}
	if opID == "" {
		err := fmt.Errorf("team must be linked to operation to view assignments")
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}
	o := model.Operation{}
	o.ID = opID
	if err := o.Populate(gid); err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}
	var b bytes.Buffer
	name, _ := teamID.Name()
	b.WriteString(fmt.Sprintf("<b>Operation: %s</b> (team: %s)\n", o.Name, name))
	b.WriteString("<b>Order / Portal / Action</b>\n")
	sort.Slice(o.Markers, func(i, j int) bool { return o.Markers[i].Order < o.Markers[j].Order })
	for _, m := range o.Markers {
		if m.State == "pending" {
			p, _ := o.PortalDetails(m.PortalID, gid)
			b.WriteString(fmt.Sprintf("<b>%d</b> / <a href=\"http://maps.google.com/?q=%s,%s\">%s</a> / %s\n", m.Order, p.Lat, p.Lon, p.Name, model.NewMarkerType(m.Type)))
		}
		msg.Text = b.String()
	}
	sendQueue <- msg
}

func gcClaim(inMsg *tgbotapi.Update) {
	msg := tgbotapi.NewMessage(inMsg.Message.Chat.ID, "")
	msg.ParseMode = "HTML"
	msg.DisableWebPagePreview = true

	gid, err := model.TelegramID(inMsg.Message.From.ID).Gid()
	if err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}

	_, opID, err := model.ChatToTeam(inMsg.Message.Chat.ID)
	if err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}
	if opID == "" {
		err := fmt.Errorf("team must be linked to operation to claim assignments")
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}
	o := model.Operation{}
	o.ID = opID
	if err := o.Populate(gid); err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}

	tokens := strings.Split(inMsg.Message.Text, " ")
	step, err := strconv.ParseInt(strings.TrimSpace(tokens[1]), 10, 16)
	if err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}
	task, err := o.GetTaskByStepNumber(int16(step))
	if err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}
	if err := task.Claim(gid); err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}

	type data struct {
		Type  string
		Name  string
		Order int16
	}
	d := data{
		Order: task.GetOrder(),
	}

	switch task.(type) {
	case model.Marker:
		m := task.(model.Marker)
		p, _ := o.PortalDetails(m.PortalID, gid)
		d.Name = p.Name
		d.Type = model.NewMarkerType(m.Type)
	case model.Link:
		l := task.(model.Link)
		p, _ := o.PortalDetails(l.From, gid)
		d.Name = p.Name
		d.Type = "link"
	}

	msg.Text, _ = templates.ExecuteLang("Claim", inMsg.Message.From.LanguageCode, d)
	sendQueue <- msg
}

func gcAcknowledge(inMsg *tgbotapi.Update) {
	msg := tgbotapi.NewMessage(inMsg.Message.Chat.ID, "")
	msg.ParseMode = "HTML"
	msg.DisableWebPagePreview = true

	gid, err := model.TelegramID(inMsg.Message.From.ID).Gid()
	if err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}

	_, opID, err := model.ChatToTeam(inMsg.Message.Chat.ID)
	if err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}
	if opID == "" {
		err := fmt.Errorf("team must be linked to operation to claim assignments")
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}
	o := model.Operation{}
	o.ID = opID
	if err := o.Populate(gid); err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}

	tokens := strings.Split(inMsg.Message.Text, " ")
	step, err := strconv.ParseInt(strings.TrimSpace(tokens[1]), 10, 16)
	if err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}
	task, err := o.GetTaskByStepNumber(int16(step))
	if err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}

	if !task.IsAssignedTo(gid) {
		err := fmt.Errorf("Task must be assigned to you to acknowledge")
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}

	if err := task.Acknowledge(); err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}
	m := task.(model.Marker)
	p, _ := o.PortalDetails(m.PortalID, gid)

	type data struct {
		Type  string
		Name  string
		Order int16
	}
	d := data{
		Type:  model.NewMarkerType(m.Type),
		Name:  p.Name,
		Order: task.GetOrder(),
	}

	msg.Text, _ = templates.ExecuteLang("Acknowledged", inMsg.Message.From.LanguageCode, d)
	sendQueue <- msg
}

func gcReject(inMsg *tgbotapi.Update) {
	msg := tgbotapi.NewMessage(inMsg.Message.Chat.ID, "")
	msg.ParseMode = "HTML"
	msg.DisableWebPagePreview = true

	gid, err := model.TelegramID(inMsg.Message.From.ID).Gid()
	if err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}

	_, opID, err := model.ChatToTeam(inMsg.Message.Chat.ID)
	if err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}
	if opID == "" {
		err := fmt.Errorf("team must be linked to operation to claim assignments")
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}
	o := model.Operation{}
	o.ID = opID
	if err := o.Populate(gid); err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}

	tokens := strings.Split(inMsg.Message.Text, " ")
	step, err := strconv.ParseInt(strings.TrimSpace(tokens[1]), 10, 16)
	if err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}
	task, err := o.GetTaskByStepNumber(int16(step))
	if err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}
	if err := task.Reject(gid); err != nil {
		log.Error(err)
		msg.Text = err.Error()
		sendQueue <- msg
		return
	}
	m := task.(model.Marker)
	p, _ := o.PortalDetails(m.PortalID, gid)

	type data struct {
		Type  string
		Name  string
		Order int16
	}
	d := data{
		Type:  model.NewMarkerType(m.Type),
		Name:  p.Name,
		Order: task.GetOrder(),
	}

	msg.Text, _ = templates.ExecuteLang("Rejected", inMsg.Message.From.LanguageCode, d)
	sendQueue <- msg
}
