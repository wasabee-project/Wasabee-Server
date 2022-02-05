package wfb

import (
	"encoding/json"
	"fmt"
	"strings"

	"firebase.google.com/go/messaging"

	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	wm "github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/model"
)

// AgentLocation alerts all appropriate teams about an agent's moving
// Do not send to topic since this hits the fanout-quota quickly
// We do the fanout manually, sending directly to tokens has a much higher quota
func AgentLocation(gid model.GoogleID) {
	if !config.IsFirebaseRunning() {
		return
	}
	if !ratelimitAgent(gid) {
		log.Debugw("skipping agent due to rate limits", "gid", gid)
		return
	}

	tokens, err := gid.FirebaseLocationTokens()
	if len(tokens) == 0 {
		return
	}

	var toSend []*messaging.Message
	var brTokens []string
	var curTeam model.TeamID
	var skipping bool

	for _, token := range tokens {
		if curTeam != token.TeamID {
			// log.Debugw("next team", "teamID", token.TeamID)
			curTeam = token.TeamID
			skipping = false
			if !ratelimitTeam(token.TeamID) {
				log.Debugw("skipping this team due to rate limits", "teamID", token.TeamID)
				skipping = true
				continue
			}
		}
		if skipping {
			log.Debugw("skipping this team", "teamID", token.TeamID)
			continue
		}

		// look through brTokens to make sure we've not already used this token
		// due to multiple shared teams

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
		if len(toSend) > 500 {
			log.Debug("got 500 messages outgoing, stopping")
			break
		}
	}

	br, err := msg.SendAll(fbctx, toSend)
	if err != nil {
		log.Error(err)
		return
	}
	processBatchResponse(br, brTokens)
	return
}

// AssignLink lets an agent know they have a new assignment on a given operation
func AssignLink(gid model.GoogleID, linkID model.TaskID, opID model.OperationID, updateID string) error {
	if !config.IsFirebaseRunning() {
		return nil
	}
	tokens, err := gid.GetFirebaseTokens()
	if err != nil {
		log.Error(err)
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
	genericMulticast(data, tokens)
	return nil
}

// AssignMarker lets an gent know they have a new assignment on a given operation
func AssignMarker(gid model.GoogleID, markerID model.TaskID, opID model.OperationID, updateID string) error {
	if !config.IsFirebaseRunning() {
		return nil
	}
	tokens, err := gid.GetFirebaseTokens()
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
	genericMulticast(data, tokens)
	return nil
}

// AssignTask lets an gent know they have a new assignment on a given operation
func AssignTask(gid model.GoogleID, taskID model.TaskID, opID model.OperationID, updateID string) error {
	if !config.IsFirebaseRunning() {
		return nil
	}
	tokens, err := gid.GetFirebaseTokens()
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
	genericMulticast(data, tokens)
	return nil
}

// MarkerStatus reports a marker update to a team/topic
func MarkerStatus(markerID model.TaskID, opID model.OperationID, teams []model.TeamID, status string, updateID string) error {
	if !config.IsFirebaseRunning() {
		return nil
	}
	if !ratelimitOp(opID) {
		return nil
	}

	data := map[string]string{
		"opID":     string(opID),
		"markerID": string(markerID),
		"msg":      status,
		"cmd":      "Marker Status Change",
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
		if _, err := msg.Send(fbctx, &m); err != nil {
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
func LinkStatus(linkID model.TaskID, opID model.OperationID, teams []model.TeamID, status string, updateID string) error {
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

		if _, err := msg.Send(fbctx, &m); err != nil {
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
func TaskStatus(taskID model.TaskID, opID model.OperationID, teams []model.TeamID, status string, updateID string) error {
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

		if _, err := msg.Send(fbctx, &m); err != nil {
			log.Error(err)
			if messaging.IsTooManyTopics(err) || messaging.IsMessageRateExceeded(err) {
				slowdown()
			}
			return err
		}
	}
	return nil
}

// addToRemote subscribes all tokens for a given agent to a team/topic
func addToRemote(g wm.GoogleID, teamID wm.TeamID) error {
	if !config.IsFirebaseRunning() {
		return nil
	}

	gid := model.GoogleID(g)
	tokens, err := gid.GetFirebaseTokens()
	if err != nil {
		log.Error(err)
		return err
	}
	if len(tokens) == 0 {
		return nil
	}

	tmr, err := msg.SubscribeToTopic(fbctx, tokens, string(teamID))
	if err != nil {
		log.Error(err)
		return err
	}
	if tmr != nil && tmr.FailureCount > 0 {
		for _, f := range tmr.Errors {
			_ = model.RemoveFirebaseToken(tokens[f.Index])
		}
	}
	return nil
}

// removeFromRemote removes an agent's subscriptions to a given topic/team
func removeFromRemote(g wm.GoogleID, teamID wm.TeamID) error {
	if !config.IsFirebaseRunning() {
		return nil
	}

	gid := model.GoogleID(g)
	tokens, err := gid.GetFirebaseTokens()
	if err != nil {
		log.Error(err)
		return err
	}
	if len(tokens) == 0 {
		return nil
	}

	tmr, err := msg.UnsubscribeFromTopic(fbctx, tokens, string(teamID))
	if err != nil {
		log.Error(err)
		return err
	}
	if tmr != nil && tmr.FailureCount > 0 {
		for _, f := range tmr.Errors {
			_ = model.RemoveFirebaseToken(tokens[f.Index])
		}
	}
	return nil
}

// sendMessage is registered with Wasabee for sending messages
func sendMessage(g wm.GoogleID, message string) (bool, error) {
	if !config.IsFirebaseRunning() {
		return false, nil
	}

	gid := model.GoogleID(g)
	tokens, err := gid.GetFirebaseTokens()
	if err != nil {
		log.Error(err)
		return false, err
	}
	if len(tokens) == 0 {
		return false, nil
	}

	data := map[string]string{
		"msg": message,
		"cmd": "Generic Message",
	}
	genericMulticast(data, tokens)
	return true, nil
}

// sendTarget sends a portal name/guid to an agent
func sendTarget(g wm.GoogleID, t wm.Target) error {
	if !config.IsFirebaseRunning() {
		return nil
	}

	gid := model.GoogleID(g)
	tokens, err := gid.GetFirebaseTokens()
	if err != nil {
		log.Error(err)
		return err
	}
	if len(tokens) == 0 {
		return nil
	}

	m, err := json.Marshal(t)
	if err != nil {
		log.Error(err)
		return err
	}

	data := map[string]string{
		"msg": string(m),
		"cmd": "Target",
	}

	genericMulticast(data, tokens)
	return nil
}

// MapChange alerts teams of the need to need to refresh map data
func MapChange(teams []model.TeamID, opID model.OperationID, updateID string) error {
	if !config.IsFirebaseRunning() {
		return nil
	}
	if !ratelimitOp(opID) {
		return nil
	}

	data := map[string]string{
		"opID":     string(opID),
		"updateID": updateID,
		"cmd":      "Map Change",
		"srv":      config.Get().HTTP.Webroot,
	}

	conditions := teamsToCondition(teams)
	multicastFantoutMutex.Lock()
	defer multicastFantoutMutex.Unlock()
	for _, condition := range conditions {
		m := messaging.Message{
			Condition: condition,
			Data:      data,
		}

		if _, err := msg.Send(fbctx, &m); err != nil {
			log.Error(err)
			if messaging.IsTooManyTopics(err) || messaging.IsMessageRateExceeded(err) {
				slowdown()
			}
			return err
		}
	}
	return nil
}

// AgentLogin alerts a team of an agent on that team logging in
func AgentLogin(teams []model.TeamID, gid model.GoogleID) error {
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

		if _, err := msg.Send(fbctx, &m); err != nil {
			log.Error(err)
			if messaging.IsTooManyTopics(err) || messaging.IsMessageRateExceeded(err) {
				slowdown()
			}
			return err
		}
	}
	return nil
}

// sendAnnounce sends a generic message to a team
func sendAnnounce(teamID wm.TeamID, a wm.Announce) error {
	if !config.IsFirebaseRunning() {
		return nil
	}
	data := map[string]string{
		"msg":    a.Text,
		"cmd":    "Generic Message",
		"opID":   string(a.OpID),
		"sender": string(a.Sender),
		"srv":    config.Get().HTTP.Webroot,
	}
	m := messaging.Message{
		Topic: string(teamID),
		Data:  data,
	}

	if _, err := msg.Send(fbctx, &m); err != nil {
		log.Error(err)
		if messaging.IsTooManyTopics(err) || messaging.IsMessageRateExceeded(err) {
			slowdown()
		}
		return err
	}
	return nil
}

// deleteOperation tells everyone (on this server) to remove a specific op
func deleteOperation(opID wm.OperationID) error {
	if !config.IsFirebaseRunning() {
		return nil
	}
	tokens, err := model.FirebaseBroadcastList()
	if err != nil {
		log.Error(err)
		return err
	}
	if len(tokens) == 0 {
		return nil
	}

	data := map[string]string{
		"cmd":  "Delete",
		"opID": string(opID),
	}

	// do this in its own worker since it might take a while
	go genericMulticast(data, tokens)
	return nil
}

// agentDeleteOperation notifies a single agent of the need to delete an operation (e.g. when removed from a team)
func agentDeleteOperation(g wm.GoogleID, opID wm.OperationID) error {
	if !config.IsFirebaseRunning() {
		return nil
	}

	gid := model.GoogleID(g)
	tokens, err := gid.GetFirebaseTokens()
	if err != nil {
		log.Error(err)
		return err
	}
	if len(tokens) == 0 {
		return nil
	}

	data := map[string]string{
		"cmd":  "Delete",
		"opID": string(opID),
	}

	genericMulticast(data, tokens)
	return nil
}

// genericMulticast sends multicast messages directly to agents, taking care of breaking into proper segments and cleaning up invalid tokens
func genericMulticast(data map[string]string, tokens []string) {
	if len(tokens) == 0 {
		return
	}
	data["srv"] = config.Get().HTTP.Webroot

	// can send up to 500 per block
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
		br, err := msg.SendMulticast(fbctx, &m)
		if err != nil {
			log.Error(err)
			return // carry on ?
		}
		log.Debugw("multicast block", "success", br.SuccessCount, "failure", br.FailureCount)
		processBatchResponse(br, subset)
	}
}

// processBatchResponse looks for invalid tokens responses and removes the offending tokens
func processBatchResponse(br *messaging.BatchResponse, tokens []string) {
	var slowed bool
	for pos, resp := range br.Responses {
		if !resp.Success {
			if messaging.IsInternal(resp.Error) || messaging.IsUnknown(resp.Error) || messaging.IsServerUnavailable(resp.Error) {
				continue
			}
			if messaging.IsRegistrationTokenNotRegistered(resp.Error) {
				_ = model.RemoveFirebaseToken(tokens[pos])
				continue
			}
			if messaging.IsMessageRateExceeded(resp.Error) && !slowed {
				slowdown()
				slowed = true // only one slowdown step per message send
				continue
			}
			log.Infow("processBatchResponse", "error", resp.Error)
		}
	}
}

// Resubscribe refreshes all the topic subscriptions for every team
func Resubscribe() {
	teams, err := model.GetAllTeams()
	if err != nil {
		log.Error(err)
		return
	}

	for _, teamID := range teams {
		tokens, err := teamID.FetchFBTokens()
		if err != nil || len(tokens) == 0 {
			continue
		}

		// TODO: fix this if we ever see it...
		if len(tokens) > 500 {
			log.Warnw("team has more than 500 tokens, only re-subscribing the first 500", "teamID", teamID, "count", len(tokens))
			tokens = tokens[:500]
		}

		// log.Debugw("resubscribing tokens", "teamID", teamID, "count", len(tokens))

		tmr, err := msg.SubscribeToTopic(fbctx, tokens, string(teamID))
		if err != nil && !messaging.IsUnknown(err) {
			log.Error(err)
			return
		}
		if tmr != nil && tmr.FailureCount > 0 {
			for _, f := range tmr.Errors {
				if !strings.Contains(f.Reason, "code: internal-error") {
					log.Debugw("removing dead firebase token", "token", tokens[f.Index][:24], "reason", f.Reason)
					_ = model.RemoveFirebaseToken(tokens[f.Index])
				} else {
					log.Debug("not removing firebase token", "reason", f.Reason)
				}
			}
		}
	}
}

func teamsToCondition(teams []model.TeamID) []string {
	var conditionSet []string

	if len(teams) == 0 {
		err := fmt.Errorf("no teams set")
		log.Info(err)
		return conditionSet
	}

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
			topic := fmt.Sprintf("'%s' in topics", string(teamID))
			condition.WriteString(topic)
		}

		log.Debug(condition.String())
		conditionSet = append(conditionSet, condition.String())
	}
	return conditionSet
}
