package wfb

import (
	// "context"
	"firebase.google.com/go/messaging"
	// "fmt"
	// "encoding/json"

	"github.com/wasabee-project/Wasabee-Server/log"
)

// wrapper types to enforce safety -- make sure we are passing the correct args in
type TeamID string
type GoogleID string
type TaskID string
type OperationID string

// Callbacks are the functions used to get/set/delete Firebase tokens in the data model
// do it this way since Go doesn't allow circular dependencies.
// model/firebase.go has an Init() to fill these in at startup
var Callbacks struct {
	GidToTokens   func(GoogleID) ([]string, error)
	StoreToken    func(GoogleID, string) error
	DeleteToken   func(string) error
	BroadcastList func() ([]string, error)
}

// 	"Generic Message", "Agent Location Change", "Map Change", "Marker Status Change", "Marker Assignment Change", "Link Status Change", "Link Assignment Change", "Login", "Delete", "Target"

// AgentLocation alerts a team to refresh agent location data
// we do not send the agent's location via firebase since it is possible to subscribe to topics (teams) via a client
// the clients must pull the server to get the updates
func AgentLocation(teamID TeamID) error {
	data := map[string]string{
		"msg": string(teamID),
		"cmd": "Agent Location Change",
		// we could send gid here for a single agent location change...
	}

	msg := messaging.Message{
		Topic: string(teamID),
		Data:  data,
	}

	_, err := config.msg.Send(config.ctx, &msg)
	if err != nil {
		log.Errorw(err.Error(), "Command", msg)
		return err
	}
	return nil
}

// AssignLink lets an agent know they have a new assignment on a given operation
func AssignLink(gid GoogleID, linkID TaskID, opID OperationID, status string) error {
	tokens, err := Callbacks.GidToTokens(gid)
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
func AssignMarker(gid GoogleID, markerID TaskID, opID OperationID, status string) error {
	tokens, err := Callbacks.GidToTokens(gid)
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
func MarkerStatus(markerID, opID, teamID, status string) error {
	data := map[string]string{
		"opID":     opID,
		"markerID": markerID,
		"msg":      status,
		"cmd":      "Marker Status Change",
	}
	msg := messaging.Message{
		Topic: teamID,
		Data:  data,
	}

	_, err := config.msg.Send(config.ctx, &msg)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// LinkStatus reports a link update to a team/topic
func LinkStatus(linkID, opID, teamID, status string) error {
	data := map[string]string{
		"opID":   opID,
		"linkID": linkID,
		"msg":    status,
		"cmd":    "Link Status Change",
	}
	msg := messaging.Message{
		Topic: teamID,
		Data:  data,
	}

	_, err := config.msg.Send(config.ctx, &msg)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// SubscribeToTopic subscribes all tokens for a given agent to a team/topic
func SubscribeToTopic(gid GoogleID, teamID TeamID) error {
	tokens, err := Callbacks.GidToTokens(gid)
	if err != nil {
		log.Error(err)
		return err
	}
	if len(tokens) == 0 {
		return nil
	}

	tmr, err := config.msg.SubscribeToTopic(config.ctx, tokens, string(teamID))
	if err != nil {
		log.Error(err)
		return err
	}
	if tmr != nil && tmr.FailureCount > 0 {
		for _, f := range tmr.Errors {
			Callbacks.DeleteToken(tokens[f.Index])
		}
	}
	return nil
}

// UnsubscribeFromTopic removes an agent's subscriptions to a given topic/team
func UnsubscribeFromTopic(gid GoogleID, teamID TeamID) error {
	tokens, err := Callbacks.GidToTokens(gid)
	if err != nil {
		log.Error(err)
		return err
	}
	if len(tokens) == 0 {
		return nil
	}

	tmr, err := config.msg.UnsubscribeFromTopic(config.ctx, tokens, string(teamID))
	if err != nil {
		log.Error(err)
		return err
	}
	if tmr != nil && tmr.FailureCount > 0 {
		for _, f := range tmr.Errors {
			Callbacks.DeleteToken(tokens[f.Index])
		}
	}
	return nil
}

// SendMessage is registered with Wasabee for sending messages
func SendMessage(gid GoogleID, message string) (bool, error) {
	tokens, err := Callbacks.GidToTokens(gid)
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
func SendTarget(gid GoogleID, message string) error {
	tokens, err := Callbacks.GidToTokens(gid)
	if err != nil {
		log.Error(err)
		return err
	}
	if len(tokens) == 0 {
		return nil
	}

	data := map[string]string{
		"msg": message,
		"cmd": "Target",
	}

	genericMulticast(data, tokens)
	return nil
}

func MapChange(teamID TeamID, opID OperationID, updateID string) error {
	data := map[string]string{
		"opID":     string(opID),
		"updateID": updateID,
		"cmd":      "Map Change",
	}

	msg := messaging.Message{
		Topic: string(teamID),
		Data:  data,
	}

	_, err := config.msg.Send(config.ctx, &msg)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func AgentLogin(teamID TeamID, gid GoogleID) error {
	data := map[string]string{
		"gid": string(gid),
		"cmd": "Login",
	}
	msg := messaging.Message{
		Topic: string(teamID),
		Data:  data,
	}

	_, err := config.msg.Send(config.ctx, &msg)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// DeleteOperation tells everyone (on this server) to remove a specific op
func DeleteOperation(opID string) error {
	tokens, err := Callbacks.BroadcastList()
	if err != nil {
		log.Error(err)
		return err
	}
	if len(tokens) == 0 {
		return nil
	}

	data := map[string]string{
		// "msg":  fb.Msg,
		"cmd":  "Delete",
		"opID": opID,
	}

	// do this in its own worker since it might take a while
	go genericMulticast(data, tokens)
	return nil
}

func AgentDeleteOperation(gid GoogleID, opID OperationID) error {
	tokens, err := Callbacks.GidToTokens(gid)
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
		br, err := config.msg.SendMulticast(config.ctx, &msg)
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
	br, err := config.msg.SendMulticast(config.ctx, &msg)
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
				Callbacks.DeleteToken(tokens[pos])
			}
		}
	}
}
