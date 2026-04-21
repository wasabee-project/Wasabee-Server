package wfb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"firebase.google.com/go/messaging"

	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	wm "github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/model"
)

// AgentLocation alerts teams about an agent's movement using request context
func AgentLocation(ctx context.Context, gid model.GoogleID) {
	if !config.IsFirebaseRunning() {
		return
	}
	if !ratelimitAgent(gid) {
		return
	}

	tokens, err := gid.FirebaseLocationTokens(ctx)
	if err != nil {
		log.Infow("firebase token load", "gid", gid, "err", err)
		return
	}
	if len(tokens) == 0 {
		return
	}

	var toSend []*messaging.Message
	var brTokens []string
	var curTeam model.TeamID
	var skipping bool

	for _, token := range tokens {
		if curTeam != token.TeamID {
			curTeam = token.TeamID
			skipping = !ratelimitTeam(token.TeamID)
		}
		if skipping {
			continue
		}

		m := messaging.Message{
			Token: token.Token,
			Data: map[string]string{
				"msg": string(token.TeamID),
				"cmd": "Agent Location Change",
				"srv": config.Get().HTTP.Webroot,
			},
		}
		toSend = append(toSend, &m)
		brTokens = append(brTokens, token.Token)
		if len(toSend) >= 500 { // Firebase limit per batch
			break
		}
	}

	if len(toSend) == 0 {
		return
	}

	br, err := msg.SendAll(ctx, toSend)
	if err != nil {
		log.Error(err)
		return
	}
	processBatchResponse(ctx, br, brTokens)
}

// AssignLink lets an agent know they have a new assignment
func AssignLink(ctx context.Context, gid model.GoogleID, linkID model.TaskID, opID model.OperationID, updateID string) error {
	if !config.IsFirebaseRunning() {
		return nil
	}
	tokens, err := gid.GetFirebaseTokens(ctx)
	if err != nil {
		return err
	}
	if len(tokens) == 0 {
		return nil
	}

	data := map[string]string{
		"opID":     string(opID),
		"linkID":   string(linkID),
		"cmd":      "Link Assignment Change",
		"updateID": updateID,
	}
	genericMulticast(ctx, data, tokens)
	return nil
}

// AssignMarker lets an gent know they have a new assignment on a given operation
func AssignMarker(ctx context.Context, gid model.GoogleID, markerID model.TaskID, opID model.OperationID, updateID string) error {
	if !config.IsFirebaseRunning() {
		return nil
	}
	tokens, err := gid.GetFirebaseTokens(ctx)
	if err != nil {
		log.Error(err)
		return err
	}
	if len(tokens) == 0 {
		return nil
	}

	data := map[string]string{
		"opID":     string(opID),
		"markerID": string(markerID),
		"cmd":      "Marker Assignment Change",
		"updateID": updateID,
	}
	genericMulticast(ctx, data, tokens)
	return nil
}

// AssignTask lets an gent know they have a new assignment on a given operation
func AssignTask(ctx context.Context, gid model.GoogleID, taskID model.TaskID, opID model.OperationID, updateID string) error {
	if !config.IsFirebaseRunning() {
		return nil
	}
	tokens, err := gid.GetFirebaseTokens(ctx)
	if err != nil {
		log.Error(err)
		return err
	}
	if len(tokens) == 0 {
		return nil
	}

	data := map[string]string{
		"opID":     string(opID),
		"taskID":   string(taskID),
		"cmd":      "Task Assignment Change",
		"updateID": updateID,
	}
	genericMulticast(ctx, data, tokens)
	return nil
}

// MarkerStatus reports a marker update to teams via topic condition
func MarkerStatus(ctx context.Context, markerID model.TaskID, opID model.OperationID, teams []model.TeamID, status string, updateID string) error {
	if !config.IsFirebaseRunning() || !ratelimitOp(opID) {
		return nil
	}

	data := map[string]string{
		"opID":     string(opID),
		"markerID": string(markerID),
		"msg":      status,
		"cmd":      "Marker Status Change",
		"updateID": updateID,
	}

	return sendToTeams(ctx, teams, data)
}

// sendToTeams is a helper to handle topic conditions with proper locking and context
func sendToTeams(ctx context.Context, teams []model.TeamID, data map[string]string) error {
	data["srv"] = config.Get().HTTP.Webroot
	conditions := teamsToCondition(teams)

	multicastFantoutMutex.Lock()
	defer multicastFantoutMutex.Unlock()

	for _, condition := range conditions {
		m := messaging.Message{
			Condition: condition,
			Data:      data,
		}
		if _, err := msg.Send(ctx, &m); err != nil {
			if messaging.IsTooManyTopics(err) || messaging.IsMessageRateExceeded(err) {
				slowdown()
			}
			log.Error(err)
			return err
		}
	}
	return nil
}

// addToRemote subscribes agent tokens to a team topic
func addToRemote(ctx context.Context, g wm.GoogleID, teamID wm.TeamID) error {
	if !config.IsFirebaseRunning() {
		return nil
	}

	gid := model.GoogleID(g)
	tokens, err := gid.GetFirebaseTokens(ctx)
	if err != nil {
		return err
	}
	if len(tokens) == 0 {
		return nil
	}

	tmr, err := msg.SubscribeToTopic(ctx, tokens, string(teamID))
	if err != nil {
		return err
	}
	if tmr != nil && tmr.FailureCount > 0 {
		for _, f := range tmr.Errors {
			_ = model.RemoveFirebaseToken(ctx, tokens[f.Index])
		}
	}
	return nil
}

// removeFromRemote unsubscribes agent tokens from a team topic
func removeFromRemote(ctx context.Context, g wm.GoogleID, teamID wm.TeamID) error {
	if !config.IsFirebaseRunning() {
		return nil
	}

	gid := model.GoogleID(g)
	tokens, err := gid.GetFirebaseTokens(ctx)
	if err != nil {
		return err
	}
	if len(tokens) == 0 {
		return nil
	}

	tmr, err := msg.UnsubscribeFromTopic(ctx, tokens, string(teamID))
	if err != nil {
		return err
	}
	if tmr != nil && tmr.FailureCount > 0 {
		for _, f := range tmr.Errors {
			_ = model.RemoveFirebaseToken(ctx, tokens[f.Index])
		}
	}
	return nil
}

// sendMessage is the generic message sender registered with Wasabee
func sendMessage(ctx context.Context, g wm.GoogleID, message string) (bool, error) {
	if !config.IsFirebaseRunning() {
		return false, nil
	}

	gid := model.GoogleID(g)
	tokens, err := gid.GetFirebaseTokens(ctx)
	if err != nil {
		return false, err
	}
	if len(tokens) == 0 {
		return false, nil
	}

	data := map[string]string{
		"msg": message,
		"cmd": "Generic Message",
	}
	genericMulticast(ctx, data, tokens)
	return true, nil
}

// MapChange alerts teams to refresh operation data
func MapChange(ctx context.Context, teams []model.TeamID, opID model.OperationID, updateID string) error {
	if !config.IsFirebaseRunning() || !ratelimitOp(opID) {
		return nil
	}

	data := map[string]string{
		"opID":     string(opID),
		"updateID": updateID,
		"cmd":      "Map Change",
	}

	return sendToTeams(ctx, teams, data)
}

// genericMulticast sends direct messages in batches of 500
func genericMulticast(ctx context.Context, data map[string]string, tokens []string) {
	if len(tokens) == 0 {
		return
	}
	data["srv"] = config.Get().HTTP.Webroot

	for len(tokens) > 0 {
		r := len(tokens)
		if r > 500 {
			r = 500
		}

		subset := tokens[:r]
		tokens = tokens[r:]
		m := messaging.MulticastMessage{
			Data:   data,
			Tokens: subset,
		}
		br, err := msg.SendMulticast(ctx, &m)
		if err != nil {
			log.Error(err)
			return
		}
		processBatchResponse(ctx, br, subset)
	}
}

// processBatchResponse handles token cleanup based on Firebase feedback
func processBatchResponse(ctx context.Context, br *messaging.BatchResponse, tokens []string) {
	var slowed bool
	for pos, resp := range br.Responses {
		if !resp.Success {
			if messaging.IsRegistrationTokenNotRegistered(resp.Error) {
				_ = model.RemoveFirebaseToken(ctx, tokens[pos])
			} else if messaging.IsMessageRateExceeded(resp.Error) && !slowed {
				slowdown()
				slowed = true
			}
		}
	}
}

// Add these back to Firebase/messaging.go

// sendTarget sends a portal name/guid to an agent
func sendTarget(ctx context.Context, g wm.GoogleID, t wm.Target) error {
	if !config.IsFirebaseRunning() {
		return nil
	}
	gid := model.GoogleID(g)
	tokens, err := gid.GetFirebaseTokens(ctx)
	if err != nil || len(tokens) == 0 {
		return err
	}

	m, err := json.Marshal(t)
	if err != nil {
		return err
	}

	data := map[string]string{
		"msg": string(m),
		"cmd": "Target",
	}
	genericMulticast(ctx, data, tokens)
	return nil
}

// sendAnnounce sends a generic message to a team
func sendAnnounce(ctx context.Context, teamID wm.TeamID, a wm.Announce) error {
	if !config.IsFirebaseRunning() {
		return nil
	}
	data := map[string]string{
		"msg":    a.Text,
		"cmd":    "Generic Message",
		"opID":   string(a.OpID),
		"sender": string(a.Sender),
	}
	m := messaging.Message{
		Topic: string(teamID),
		Data:  data,
	}
	_, err := msg.Send(ctx, &m)
	return err
}

// agentDeleteOperation notifies a single agent to delete an operation
func agentDeleteOperation(ctx context.Context, g wm.GoogleID, opID wm.OperationID) error {
	if !config.IsFirebaseRunning() {
		return nil
	}
	gid := model.GoogleID(g)
	tokens, err := gid.GetFirebaseTokens(ctx)
	if err != nil || len(tokens) == 0 {
		return err
	}

	data := map[string]string{
		"cmd":  "Delete",
		"opID": string(opID),
	}
	genericMulticast(ctx, data, tokens)
	return nil
}

// deleteOperation tells everyone to remove a specific op
func deleteOperation(ctx context.Context, opID wm.OperationID) error {
	if !config.IsFirebaseRunning() {
		return nil
	}
	tokens, err := model.FirebaseBroadcastList(ctx)
	if err != nil || len(tokens) == 0 {
		return err
	}

	data := map[string]string{
		"cmd":  "Delete",
		"opID": string(opID),
	}
	go genericMulticast(context.Background(), data, tokens)
	return nil
}

func teamsToCondition(teams []model.TeamID) []string {
	var conditionSet []string
	for len(teams) > 0 {
		r := len(teams)
		if r > 5 {
			r = 5
		}
		subset := teams[:r]
		teams = teams[r:]

		var condition strings.Builder
		for i, teamID := range subset {
			if i > 0 {
				condition.WriteString(" || ")
			}
			fmt.Fprintf(&condition, "'%s' in topics", string(teamID))
		}
		conditionSet = append(conditionSet, condition.String())
	}
	return conditionSet
}

func Resubscribe(ctx context.Context) {
	teams, err := model.GetAllTeams(ctx)
	if err != nil {
		log.Error(err)
		return
	}

	for _, teamID := range teams {
		tokens, err := teamID.FetchFBTokens(ctx)
		if err != nil || len(tokens) == 0 {
			continue
		}

		// TODO: fix this if we ever see it...
		if len(tokens) > 1000 {
			log.Warnw("team has more than 1000 tokens, only re-subscribing the first 1000", "teamID", teamID, "count", len(tokens), "scot is lazy", "AF")
			tokens = tokens[:1000]
		}

		// log.Debugw("resubscribing tokens", "teamID", teamID, "count", len(tokens))

		tmr, err := msg.SubscribeToTopic(ctx, tokens, string(teamID))
		if err != nil && !messaging.IsUnknown(err) {
			log.Error(err)
			return
		}
		if tmr != nil && tmr.FailureCount > 0 {
			for _, f := range tmr.Errors {
				if !strings.Contains(f.Reason, "code: internal-error") {
					log.Debugw("removing dead firebase token", "token", tokens[f.Index][:24], "reason", f.Reason)
					_ = model.RemoveFirebaseToken(ctx, tokens[f.Index])
				} else {
					log.Debugw("not removing firebase token", "reason", f.Reason)
				}
			}
		}
	}
}

func AgentLogin(ctx context.Context, teams []model.TeamID, gid model.GoogleID) error {
	if !config.IsFirebaseRunning() {
		return nil
	}

	data := map[string]string{
		"gid": string(gid),
		"cmd": "Login",
		"srv": config.Get().HTTP.Webroot,
	}

	conditions := teamsToCondition(teams)
	multicastFantoutMutex.Lock()
	defer multicastFantoutMutex.Unlock()
	for _, condition := range conditions {
		m := messaging.Message{
			Condition: condition,
			Data:      data,
		}

		if _, err := msg.Send(context.Background(), &m); err != nil {
			log.Error(err)
			if messaging.IsTooManyTopics(err) || messaging.IsMessageRateExceeded(err) {
				slowdown()
			}
			return err
		}
	}
	return nil
}

// LinkStatus reports a link update to a team/topic
func LinkStatus(ctx context.Context, linkID model.TaskID, opID model.OperationID, teams []model.TeamID, status string, updateID string) error {
	if !config.IsFirebaseRunning() {
		return nil
	}
	if !ratelimitOp(opID) {
		return nil
	}

	data := map[string]string{
		"opID":     string(opID),
		"linkID":   string(linkID),
		"msg":      status,
		"cmd":      "Link Status Change",
		"srv":      config.Get().HTTP.Webroot,
		"updateID": updateID,
	}

	conditions := teamsToCondition(teams)
	multicastFantoutMutex.Lock()
	defer multicastFantoutMutex.Unlock()
	for _, condition := range conditions {
		m := messaging.Message{
			Condition: condition,
			Data:      data,
		}

		if _, err := msg.Send(ctx, &m); err != nil {
			log.Error(err)
			if messaging.IsTooManyTopics(err) || messaging.IsMessageRateExceeded(err) {
				slowdown()
			}
			return err
		}
	}
	return nil
}

// TaskStatus reports a task update to a team/topic
func TaskStatus(ctx context.Context, taskID model.TaskID, opID model.OperationID, teams []model.TeamID, status string, updateID string) error {
	if !config.IsFirebaseRunning() {
		return nil
	}
	if !ratelimitOp(opID) {
		return nil
	}

	data := map[string]string{
		"opID":     string(opID),
		"taskID":   string(taskID),
		"msg":      status,
		"cmd":      "Task Status Change",
		"srv":      config.Get().HTTP.Webroot,
		"updateID": updateID,
	}

	conditions := teamsToCondition(teams)
	multicastFantoutMutex.Lock()
	defer multicastFantoutMutex.Unlock()
	for _, condition := range conditions {
		m := messaging.Message{
			Condition: condition,
			Data:      data,
		}

		if _, err := msg.Send(ctx, &m); err != nil {
			log.Error(err)
			if messaging.IsTooManyTopics(err) || messaging.IsMessageRateExceeded(err) {
				slowdown()
			}
			return err
		}
	}
	return nil
}
