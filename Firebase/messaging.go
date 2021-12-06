package wfb

import (
	"encoding/json"

	"firebase.google.com/go/messaging"

	"github.com/wasabee-project/Wasabee-Server/log"
	wm "github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/model"
)

// 	"Generic Message", "Agent Location Change", "Map Change", "Marker Status Change", "Marker Assignment Change", "Link Status Change", "Link Assignment Change", "Login", "Delete", "Target"

// AgentLocation alerts a team to refresh agent location data
// we do not send the agent's location via firebase since it is possible to subscribe to topics (teams) via a client
// the clients must pull the server to get the updates
func AgentLocation(teamID model.TeamID) error {
	if !c.running {
		return nil
	}
	data := map[string]string{
		"msg": string(teamID),
		"cmd": "Agent Location Change",
		// we could send gid here for a single agent location change...
	}

	msg := messaging.Message{
		Topic: string(teamID),
		Data:  data,
	}

	_, err := c.msg.Send(c.ctx, &msg)
	if err != nil {
		log.Errorw(err.Error(), "Command", msg)
		return err
	}
	return nil
}

// AssignLink lets an agent know they have a new assignment on a given operation
func AssignLink(gid model.GoogleID, linkID model.TaskID, opID model.OperationID, status string) error {
	if !c.running {
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
		"opID":   string(opID),
		"linkID": string(linkID),
		"msg":    status,
		"cmd":    "Link Assignment Change",
	}
	genericMulticast(data, tokens)
	return nil
}

// AssignMarker lets an gent know they have a new assignment on a given operation
func AssignMarker(gid model.GoogleID, markerID model.TaskID, opID model.OperationID, status string) error {
	if !c.running {
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
		"msg":      status,
		"cmd":      "Marker Assignment Change",
	}
	genericMulticast(data, tokens)
	return nil
}

// MarkerStatus reports a marker update to a team/topic
func MarkerStatus(markerID model.TaskID, opID model.OperationID, teamID model.TeamID, status string) error {
	if !c.running {
		return nil
	}
	data := map[string]string{
		"opID":     string(opID),
		"markerID": string(markerID),
		"msg":      status,
		"cmd":      "Marker Status Change",
	}
	msg := messaging.Message{
		Topic: string(teamID),
		Data:  data,
	}

	_, err := c.msg.Send(c.ctx, &msg)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// LinkStatus reports a link update to a team/topic
func LinkStatus(linkID model.TaskID, opID model.OperationID, teamID model.TeamID, status string) error {
	if !c.running {
		return nil
	}
	data := map[string]string{
		"opID":   string(opID),
		"linkID": string(linkID),
		"msg":    status,
		"cmd":    "Link Status Change",
	}
	msg := messaging.Message{
		Topic: string(teamID),
		Data:  data,
	}

	_, err := c.msg.Send(c.ctx, &msg)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// AddToRemote subscribes all tokens for a given agent to a team/topic
func AddToRemote(g wm.GoogleID, teamID wm.TeamID) error {
	gid := model.GoogleID(g)

	if !c.running {
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

	tmr, err := c.msg.SubscribeToTopic(c.ctx, tokens, string(teamID))
	if err != nil {
		log.Error(err)
		return err
	}
	if tmr != nil && tmr.FailureCount > 0 {
		for _, f := range tmr.Errors {
			model.RemoveFirebaseToken(tokens[f.Index])
		}
	}
	return nil
}

// RemoveFromRemote removes an agent's subscriptions to a given topic/team
func RemoveFromRemote(g wm.GoogleID, teamID wm.TeamID) error {
	gid := model.GoogleID(g)

	if !c.running {
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

	tmr, err := c.msg.UnsubscribeFromTopic(c.ctx, tokens, string(teamID))
	if err != nil {
		log.Error(err)
		return err
	}
	if tmr != nil && tmr.FailureCount > 0 {
		for _, f := range tmr.Errors {
			model.RemoveFirebaseToken(tokens[f.Index])
		}
	}
	return nil
}

// SendMessage is registered with Wasabee for sending messages
func SendMessage(g wm.GoogleID, message string) (bool, error) {
	gid := model.GoogleID(g)

	if !c.running {
		return false, nil
	}
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

// SendTarget sends a portal name/guid to an agent
func SendTarget(g wm.GoogleID, t wm.Target) error {
	gid := model.GoogleID(g)

	if !c.running {
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

	message, err := json.Marshal(t)
	if err != nil {
		log.Error(err)
		return err
	}

	data := map[string]string{
		"msg": string(message),
		"cmd": "Target",
	}

	genericMulticast(data, tokens)
	return nil
}

func MapChange(teamID model.TeamID, opID model.OperationID, updateID string) error {
	if !c.running {
		return nil
	}
	data := map[string]string{
		"opID":     string(opID),
		"updateID": updateID,
		"cmd":      "Map Change",
	}

	msg := messaging.Message{
		Topic: string(teamID),
		Data:  data,
	}

	_, err := c.msg.Send(c.ctx, &msg)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func AgentLogin(teamID model.TeamID, gid model.GoogleID) error {
	if !c.running {
		return nil
	}
	data := map[string]string{
		"gid": string(gid),
		"cmd": "Login",
	}
	msg := messaging.Message{
		Topic: string(teamID),
		Data:  data,
	}

	_, err := c.msg.Send(c.ctx, &msg)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func SendAnnounce(teamID wm.TeamID, message string) error {
	if !c.running {
		return nil
	}
	data := map[string]string{
		"msg": message,
		"cmd": "Generic Message",
	}
	msg := messaging.Message{
		Topic: string(teamID),
		Data:  data,
	}

	_, err := c.msg.Send(c.ctx, &msg)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// DeleteOperation tells everyone (on this server) to remove a specific op
func DeleteOperation(opID wm.OperationID) error {
	if !c.running {
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

func AgentDeleteOperation(g wm.GoogleID, opID wm.OperationID) error {
	gid := model.GoogleID(g)

	if !c.running {
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
		"cmd":  "Delete",
		"opID": string(opID),
	}

	genericMulticast(data, tokens)
	return nil
}

func genericMulticast(data map[string]string, tokens []string) {
	if len(tokens) == 0 {
		return
	}

	// can send up to 500 per block
	for len(tokens) > 500 {
		subset := tokens[:500]
		tokens = tokens[500:]
		msg := messaging.MulticastMessage{
			Data:   data,
			Tokens: subset,
		}
		br, err := c.msg.SendMulticast(c.ctx, &msg)
		if err != nil {
			log.Error(err)
			// carry on
		}
		// log.Debugw("multicast block", "success", br.SuccessCount, "failure", br.FailureCount)
		processBatchResponse(br, subset)
	}

	msg := messaging.MulticastMessage{
		Data:   data,
		Tokens: tokens,
	}
	br, err := c.msg.SendMulticast(c.ctx, &msg)
	if err != nil {
		log.Error(err)
		// carry on
	}
	// log.Debugw("final multicast block", "success", br.SuccessCount, "failure", br.FailureCount)
	processBatchResponse(br, tokens)
}

func processBatchResponse(br *messaging.BatchResponse, tokens []string) {
	for pos, resp := range br.Responses {
		if !resp.Success {
			if messaging.IsRegistrationTokenNotRegistered(resp.Error) {
				model.RemoveFirebaseToken(tokens[pos])
			}
		}
	}
}
