package wfb

import (
	"encoding/json"

	"firebase.google.com/go/messaging"

	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	wm "github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/model"
)

// AgentLocation alerts a team to refresh agent location data
// we do not send the agent's location via firebase since it is possible to subscribe to topics (teams) via a client
// the clients must pull the server to get the updates
func AgentLocation(teamID model.TeamID) error {
	if !config.IsFirebaseRunning() {
		return nil
	}
	if !ratelimitTeam(teamID) {
		return nil
	}

	data := map[string]string{
		"msg": string(teamID),
		"cmd": "Agent Location Change",
		"srv": config.Get().HTTP.Webroot,
		// we could send gid here for a single agent location change...
	}

	m := messaging.Message{
		Topic: string(teamID),
		Data:  data,
	}

	if _, err := msg.Send(fbctx, &m); err != nil {
		log.Errorw(err.Error(), "Command", m)
		return err
	}
	return nil
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
func MarkerStatus(markerID model.TaskID, opID model.OperationID, teamID model.TeamID, status string, updateID string) error {
	if !config.IsFirebaseRunning() {
		return nil
	}
	if !ratelimitOp(teamID, opID) {
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
	m := messaging.Message{
		Topic: string(teamID),
		Data:  data,
	}

	if _, err := msg.Send(fbctx, &m); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// LinkStatus reports a link update to a team/topic
func LinkStatus(linkID model.TaskID, opID model.OperationID, teamID model.TeamID, status string, updateID string) error {
	if !config.IsFirebaseRunning() {
		return nil
	}
	if !ratelimitOp(teamID, opID) {
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
	m := messaging.Message{
		Topic: string(teamID),
		Data:  data,
	}

	if _, err := msg.Send(fbctx, &m); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// TaskStatus reports a task update to a team/topic
func TaskStatus(taskID model.TaskID, opID model.OperationID, teamID model.TeamID, status string, updateID string) error {
	if !config.IsFirebaseRunning() {
		return nil
	}
	if !ratelimitOp(teamID, opID) {
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
	m := messaging.Message{
		Topic: string(teamID),
		Data:  data,
	}

	if _, err := msg.Send(fbctx, &m); err != nil {
		log.Error(err)
		return err
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

// MapChange alerts a team of the need to need to refresh map data
func MapChange(teamID model.TeamID, opID model.OperationID, updateID string) error {
	if !config.IsFirebaseRunning() {
		return nil
	}
	if !ratelimitOp(teamID, opID) {
		return nil
	}

	data := map[string]string{
		"opID":     string(opID),
		"updateID": updateID,
		"cmd":      "Map Change",
		"srv":      config.Get().HTTP.Webroot,
	}

	m := messaging.Message{
		Topic: string(teamID),
		Data:  data,
	}

	if _, err := msg.Send(fbctx, &m); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// AgentLogin alerts a team of an agent on that team logging in
func AgentLogin(teamID model.TeamID, gid model.GoogleID) error {
	if !config.IsFirebaseRunning() {
		return nil
	}
	data := map[string]string{
		"gid": string(gid),
		"cmd": "Login",
		"srv": config.Get().HTTP.Webroot,
	}
	m := messaging.Message{
		Topic: string(teamID),
		Data:  data,
	}

	if _, err := msg.Send(fbctx, &m); err != nil {
		log.Error(err)
		return err
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

	// can send up to 500 per block
	for len(tokens) > 500 {
		subset := tokens[:500]
		tokens = tokens[500:]
		m := messaging.MulticastMessage{
			Data:   data,
			Tokens: subset,
		}
		br, err := msg.SendMulticast(fbctx, &m)
		if err != nil {
			log.Error(err)
			// carry on
		}
		// log.Debugw("multicast block", "success", br.SuccessCount, "failure", br.FailureCount)
		processBatchResponse(br, subset)
	}

	data["srv"] = config.Get().HTTP.Webroot

	m := messaging.MulticastMessage{
		Data:   data,
		Tokens: tokens,
	}
	br, err := msg.SendMulticast(fbctx, &m)
	if err != nil {
		log.Error(err)
		// carry on
	}
	// log.Debugw("final multicast block", "success", br.SuccessCount, "failure", br.FailureCount)
	processBatchResponse(br, tokens)
}

// processBatchResponse looks for invalid tokens responses and removes the offending tokens
func processBatchResponse(br *messaging.BatchResponse, tokens []string) {
	for pos, resp := range br.Responses {
		if !resp.Success {
			if messaging.IsRegistrationTokenNotRegistered(resp.Error) {
				_ = model.RemoveFirebaseToken(tokens[pos])
			} else {
				log.Warn(resp.Error)
			}
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

		// log.Debugw("resubscribing tokens", "teamID", teamID, "count", len(tokens))

		tmr, err := msg.SubscribeToTopic(fbctx, tokens, string(teamID))
		if err != nil {
			log.Error(err)
			return
		}
		if tmr != nil && tmr.FailureCount > 0 {
			for _, f := range tmr.Errors {
				log.Debugw("removing dead firebase token", "token", tokens[f.Index])
				_ = model.RemoveFirebaseToken(tokens[f.Index])
			}
		}
	}
}
