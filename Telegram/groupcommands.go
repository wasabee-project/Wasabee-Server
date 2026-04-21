package wtg

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/templates"
)

// newGroupResponse creates a message geared for group chats
func newGroupResponse(chatID int64) tgbotapi.MessageConfig {
	msg := tgbotapi.NewMessage(chatID, "")
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true
	// Typically, we don't send the baseKbd to groups to avoid UI clutter
	return msg
}

// sendError helper to reduce boilerplate in error paths
func sendError(res *tgbotapi.MessageConfig, err error) {
	log.Error(err)
	res.Text = fmt.Sprintf("<b>Error:</b> %s", err.Error())
	sendQueue <- *res
}

func gcUnlink(ctx context.Context, inMsg *tgbotapi.Update) {
	res := newGroupResponse(inMsg.Message.Chat.ID)

	gid, err := model.TelegramID(inMsg.Message.From.ID).Gid(ctx)
	if err != nil {
		sendError(&res, err)
		return
	}

	teamID, _, err := model.ChatToTeam(ctx, inMsg.Message.Chat.ID)
	if err != nil {
		sendError(&res, err)
		return
	}

	owns, err := gid.OwnsTeam(ctx, teamID)
	if err != nil || !owns {
		if err == nil {
			err = fmt.Errorf("only team owner can unlink the team")
		}
		sendError(&res, err)
		return
	}

	if err := teamID.UnlinkFromTelegramChat(ctx); err != nil {
		sendError(&res, err)
		return
	}

	res.Text, _ = templates.ExecuteLang("Unlinked", inMsg.Message.From.LanguageCode, nil)
	sendQueue <- res
}

func gcLink(ctx context.Context, inMsg *tgbotapi.Update) {
	res := newGroupResponse(inMsg.Message.Chat.ID)

	gid, err := model.TelegramID(inMsg.Message.From.ID).Gid(ctx)
	if err != nil {
		sendError(&res, err)
		return
	}

	args := strings.Fields(inMsg.Message.CommandArguments())
	if len(args) < 1 {
		res.Text, _ = templates.ExecuteLang("SingleTeam", inMsg.Message.From.LanguageCode, nil)
		sendQueue <- res
		return
	}

	teamID := model.TeamID(args[0])
	var opID model.OperationID
	if len(args) >= 2 {
		opID = model.OperationID(args[1])
	}

	owns, err := gid.OwnsTeam(ctx, teamID)
	if err != nil || !owns {
		if err == nil {
			res.Text, _ = templates.ExecuteLang("onlyOwners", inMsg.Message.From.LanguageCode, nil)
			sendQueue <- res
		} else {
			sendError(&res, err)
		}
		return
	}

	if err := teamID.LinkToTelegramChat(ctx, model.TelegramID(inMsg.Message.Chat.ID), opID); err != nil {
		sendError(&res, err)
		return
	}

	res.Text, _ = templates.ExecuteLang("Linked", inMsg.Message.From.LanguageCode, nil)
	sendQueue <- res
}

func gcStatus(ctx context.Context, inMsg *tgbotapi.Update) {
	res := newGroupResponse(inMsg.Message.Chat.ID)

	teamID, opID, err := model.ChatToTeam(ctx, inMsg.Message.Chat.ID)
	if err != nil {
		res.Text = err.Error()
		sendQueue <- res
		return
	}

	name, _ := teamID.Name(ctx)
	d := struct {
		OPStat   *model.OpStat
		TeamName string
		TeamID   model.TeamID
	}{
		TeamName: name,
		TeamID:   teamID,
	}

	if opID != "" {
		d.OPStat, _ = opID.Stat(ctx)
	}

	res.Text, _ = templates.ExecuteLang("ChatLinkStatus", inMsg.Message.From.LanguageCode, d)
	sendQueue <- res
}

func gcClaim(ctx context.Context, inMsg *tgbotapi.Update) {
	res := newGroupResponse(inMsg.Message.Chat.ID)

	gid, err := model.TelegramID(inMsg.Message.From.ID).Gid(ctx)
	if err != nil {
		sendError(&res, err)
		return
	}

	_, opID, err := model.ChatToTeam(ctx, inMsg.Message.Chat.ID)
	if err != nil || opID == "" {
		if err == nil {
			err = fmt.Errorf("team must be linked to operation to claim assignments")
		}
		sendError(&res, err)
		return
	}

	arg := inMsg.Message.CommandArguments()
	if arg == "" {
		sendError(&res, fmt.Errorf("specify the task step #"))
		return
	}

	step, err := strconv.ParseInt(strings.TrimSpace(arg), 10, 16)
	if err != nil {
		sendError(&res, err)
		return
	}

	o := model.Operation{ID: opID}
	if err := o.Populate(ctx, gid); err != nil {
		sendError(&res, err)
		return
	}

	task, err := o.GetTaskByStepNumber(int16(step))
	if err != nil {
		sendError(&res, err)
		return
	}

	if err := task.Claim(ctx, gid); err != nil {
		sendError(&res, err)
		return
	}

	d := struct {
		Type  string
		Name  string
		Order int16
	}{
		Order: task.GetOrder(),
	}

	switch t := task.(type) {
	case *model.Marker:
		p, _ := o.PortalDetails(ctx, t.PortalID, gid)
		d.Name = p.Name
		d.Type = model.NewMarkerType(t.Type)
	case *model.Link:
		p, _ := o.PortalDetails(ctx, t.From, gid)
		d.Name = p.Name
		d.Type = "link"
	}

	res.Text, _ = templates.ExecuteLang("Claim", inMsg.Message.From.LanguageCode, d)
	sendQueue <- res
}

func gcAssigned(ctx context.Context, inMsg *tgbotapi.Update) {
	res := newGroupResponse(inMsg.Message.Chat.ID)
	from := inMsg.Message.From

	gid, err := model.TelegramID(from.ID).Gid(ctx)
	if err != nil {
		sendError(&res, err)
		return
	}

	teamID, opID, err := model.ChatToTeam(ctx, inMsg.Message.Chat.ID)
	if err != nil || opID == "" {
		if err == nil {
			err = fmt.Errorf("team must be linked to operation to view assignments")
		}
		sendError(&res, err)
		return
	}

	// Handle optional filter argument
	var filterGid model.GoogleID
	if arg := inMsg.Message.CommandArguments(); arg != "" {
		filterGid, err = model.SearchAgentName(ctx, strings.TrimSpace(arg))
		if err != nil {
			log.Error(err)
			filterGid = "0" // effectively filters everything if name not found
		}
	}

	o := model.Operation{ID: opID}
	if err := o.Populate(ctx, gid); err != nil {
		sendError(&res, err)
		return
	}

	type data struct {
		OpName           string
		TeamName         string
		MarkersFormatted []string
	}
	name, _ := teamID.Name(ctx)
	d := data{
		OpName:   o.Name,
		TeamName: name,
	}

	sort.Slice(o.Markers, func(i, j int) bool { return o.Markers[i].Order < o.Markers[j].Order })

	for _, m := range o.Markers {
		if filterGid != "" && !m.IsAssignedTo(ctx, filterGid) {
			continue
		}
		if m.State == "pending" {
			continue
		}

		p, _ := o.PortalDetails(ctx, m.PortalID, gid)
		a, _ := m.AssignedTo.IngressName(ctx)
		if tg, _ := m.AssignedTo.TelegramName(ctx); tg != "" {
			a = fmt.Sprintf("@%s", tg)
		}

		strikeStart, strikeEnd := "", ""
		if m.State == "completed" {
			strikeStart, strikeEnd = "<strike>", "</strike>"
		}

		entry := fmt.Sprintf("%d / %s<a href=\"https://www.google.com/maps/search/?api=1&query=%s,%s\">%s</a> / %s / %s / %s%s\n",
			m.Order, strikeStart, p.Lat, p.Lon, p.Name, model.NewMarkerType(m.Type), a, m.State, strikeEnd)

		d.MarkersFormatted = append(d.MarkersFormatted, entry)
	}

	res.Text, _ = templates.ExecuteLang("assignments", from.LanguageCode, d)
	sendQueue <- res
}

func gcUnassigned(ctx context.Context, inMsg *tgbotapi.Update) {
	res := newGroupResponse(inMsg.Message.Chat.ID)
	from := inMsg.Message.From

	gid, err := model.TelegramID(from.ID).Gid(ctx)
	if err != nil {
		sendError(&res, err)
		return
	}

	teamID, opID, err := model.ChatToTeam(ctx, inMsg.Message.Chat.ID)
	if err != nil || opID == "" {
		if err == nil {
			err = fmt.Errorf("team must be linked to operation to view unassigned tasks")
		}
		sendError(&res, err)
		return
	}

	o := model.Operation{ID: opID}
	if err := o.Populate(ctx, gid); err != nil {
		sendError(&res, err)
		return
	}

	type data struct {
		OpName           string
		TeamName         string
		MarkersFormatted []string
	}
	tName, _ := teamID.Name(ctx)
	d := data{OpName: o.Name, TeamName: tName}

	sort.Slice(o.Markers, func(i, j int) bool { return o.Markers[i].Order < o.Markers[j].Order })

	for _, m := range o.Markers {
		if m.State == "pending" {
			p, _ := o.PortalDetails(ctx, m.PortalID, gid)
			entry := fmt.Sprintf("<b>%d</b> / <a href=\"https://www.google.com/maps/search/?api=1&query=%s,%s\">%s</a> / %s\n",
				m.Order, p.Lat, p.Lon, p.Name, model.NewMarkerType(m.Type))
			d.MarkersFormatted = append(d.MarkersFormatted, entry)
		}
	}

	res.Text, _ = templates.ExecuteLang("assignments", from.LanguageCode, d)
	sendQueue <- res
}

func gcAcknowledge(ctx context.Context, inMsg *tgbotapi.Update) {
	res := newGroupResponse(inMsg.Message.Chat.ID)
	from := inMsg.Message.From

	gid, err := model.TelegramID(from.ID).Gid(ctx)
	if err != nil {
		sendError(&res, err)
		return
	}

	_, opID, err := model.ChatToTeam(ctx, inMsg.Message.Chat.ID)
	if err != nil || opID == "" {
		sendError(&res, fmt.Errorf("team link required"))
		return
	}

	step, err := strconv.ParseInt(strings.TrimSpace(inMsg.Message.CommandArguments()), 10, 16)
	if err != nil {
		sendError(&res, fmt.Errorf("invalid step number"))
		return
	}

	o := model.Operation{ID: opID}
	task, err := o.GetTaskByStepNumber(int16(step))
	if err != nil {
		sendError(&res, err)
		return
	}

	if !task.IsAssignedTo(ctx, gid) {
		sendError(&res, fmt.Errorf("task is not assigned to you"))
		return
	}

	if err := task.Acknowledge(ctx); err != nil {
		sendError(&res, err)
		return
	}

	// Template data setup
	d := struct {
		Type  string
		Name  string
		Order int16
	}{
		Order: task.GetOrder(),
	}

	if m, ok := task.(*model.Marker); ok {
		p, _ := o.PortalDetails(ctx, m.PortalID, gid)
		d.Name = p.Name
		d.Type = model.NewMarkerType(m.Type)
	}

	res.Text, _ = templates.ExecuteLang("Acknowledged", from.LanguageCode, d)
	sendQueue <- res
}

func gcReject(ctx context.Context, inMsg *tgbotapi.Update) {
	res := newGroupResponse(inMsg.Message.Chat.ID)
	from := inMsg.Message.From

	gid, err := model.TelegramID(from.ID).Gid(ctx)
	if err != nil {
		sendError(&res, err)
		return
	}

	_, opID, err := model.ChatToTeam(ctx, inMsg.Message.Chat.ID)
	if err != nil || opID == "" {
		sendError(&res, fmt.Errorf("team link required"))
		return
	}

	step, err := strconv.ParseInt(strings.TrimSpace(inMsg.Message.CommandArguments()), 10, 16)
	if err != nil {
		sendError(&res, fmt.Errorf("invalid step number"))
		return
	}

	o := model.Operation{ID: opID}
	task, err := o.GetTaskByStepNumber(int16(step))
	if err != nil {
		sendError(&res, err)
		return
	}

	if err := task.Reject(ctx, gid); err != nil {
		sendError(&res, err)
		return
	}

	d := struct {
		Type  string
		Name  string
		Order int16
	}{
		Order: task.GetOrder(),
	}

	if m, ok := task.(*model.Marker); ok {
		p, _ := o.PortalDetails(ctx, m.PortalID, gid)
		d.Name = p.Name
		d.Type = model.NewMarkerType(m.Type)
	}

	res.Text, _ = templates.ExecuteLang("Rejected", from.LanguageCode, d)
	sendQueue <- res
}
