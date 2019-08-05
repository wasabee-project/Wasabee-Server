package wasabeefirebase

import (
	"context"
	"firebase.google.com/go/messaging"
	"fmt"
	"github.com/wasabee-project/Wasabee-Server"
)

// SendMessage is registered with Wasabee for sending messages
func SendMessage(gid wasabee.GoogleID, message string) (bool, error) {
	return false, nil
}

func agentLocationChange(ctx context.Context, c *messaging.Client, fb wasabee.FirebaseCmd) error {
	if fb.TeamID == "" {
		err := fmt.Errorf("only send location changes to teams")
		wasabee.Log.Error(err)
		return err
	}

	data := map[string]string{
		"gid": string(fb.Gid),
		"msg": fb.Cmd.String(),
		"cmd": fb.Cmd.String(),
	}

	msg := messaging.Message{
		Topic: string(fb.TeamID),
		Data:  data,
	}

	_, err := c.Send(ctx, &msg)
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	return nil
}

func markerStatusChange(ctx context.Context, c *messaging.Client, fb wasabee.FirebaseCmd) error {
	if fb.TeamID == "" {
		err := fmt.Errorf("only send status changes to teams")
		wasabee.Log.Error(err)
		return err
	}

	data := map[string]string{
		"opID":     string(fb.OpID),
		"markerID": fb.ObjID,
		"msg":      fb.Msg,
		"cmd":      fb.Cmd.String(),
	}
	msg := messaging.Message{
		Topic: string(fb.TeamID),
		Data:  data,
	}

	_, err := c.Send(ctx, &msg)
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	return nil
}

func markerAssignmentChange(ctx context.Context, c *messaging.Client, fb wasabee.FirebaseCmd) error {
	if fb.Gid == "" {
		return nil
	}

	tokens, err := fb.Gid.FirebaseTokens()
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}

	for _, token := range tokens {
		if token == "" {
			continue
		}

		notif := messaging.Notification{
			Title: "Marker Assignment",
			Body:  "You have been assigned a new marker",
		}

		data := map[string]string{
			"opID":     string(fb.OpID),
			"markerID": fb.ObjID,
			"msg":      fb.Msg,
			"cmd":      fb.Cmd.String(),
		}

		msg := messaging.Message{
			Token:        token,
			Data:         data,
			Notification: &notif,
		}

		_, err = c.Send(ctx, &msg)
		if err != nil {
			wasabee.Log.Error(err)
			return err
		}
	}
	return nil
}

func mapChange(ctx context.Context, c *messaging.Client, fb wasabee.FirebaseCmd) error {
	if fb.TeamID == "" {
		err := fmt.Errorf("only send status changes to teams")
		wasabee.Log.Error(err)
		return err
	}

	data := map[string]string{
		"opID": string(fb.OpID),
		"msg":  fb.Msg,
		"cmd":  fb.Cmd.String(),
	}
	msg := messaging.Message{
		Topic: string(fb.TeamID),
		Data:  data,
	}

	_, err := c.Send(ctx, &msg)
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	return nil
}

func linkStatusChange(ctx context.Context, c *messaging.Client, fb wasabee.FirebaseCmd) error {
	if fb.TeamID == "" {
		err := fmt.Errorf("only send status changes to teams")
		wasabee.Log.Error(err)
		return err
	}

	data := map[string]string{
		"opID":   string(fb.OpID),
		"linkID": fb.ObjID,
		"msg":    fb.Msg,
		"cmd":    fb.Cmd.String(),
	}
	msg := messaging.Message{
		Topic: string(fb.TeamID),
		Data:  data,
	}

	_, err := c.Send(ctx, &msg)
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	return nil
}

func linkAssignmentChange(ctx context.Context, c *messaging.Client, fb wasabee.FirebaseCmd) error {
	if fb.Gid == "" {
		return nil
	}

	tokens, err := fb.Gid.FirebaseTokens()
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}

	for _, token := range tokens {
		if token == "" {
			continue
		}

		notif := messaging.Notification{
			Title: "Link Assignment",
			Body:  "You have been assigned a new link",
		}

		data := map[string]string{
			"opID":   string(fb.OpID),
			"linkID": fb.ObjID,
			"msg":    fb.Msg,
			"cmd":    fb.Cmd.String(),
		}

		msg := messaging.Message{
			Token:        token,
			Data:         data,
			Notification: &notif,
		}

		_, err = c.Send(ctx, &msg)
		if err != nil {
			wasabee.Log.Error(err)
			return err
		}
	}
	return nil
}

func subscribeToTeam(ctx context.Context, c *messaging.Client, fb wasabee.FirebaseCmd) error {
	if fb.Gid == "" {
		return nil
	}
	if fb.TeamID == "" {
		return nil
	}

	tokens, err := fb.Gid.FirebaseTokens()
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}

	var tmr *messaging.TopicManagementResponse
	if fb.Msg == "subscribe" {
		tmr, err = c.SubscribeToTopic(ctx, tokens, string(fb.TeamID))
		wasabee.Log.Debugf("subscribed %s to %s (%s)", fb.Gid, fb.TeamID, tokens)
	} else {
		tmr, err = c.UnsubscribeFromTopic(ctx, tokens, string(fb.TeamID))
		wasabee.Log.Debugf("unsubscribed %s from %s (%s)", fb.Gid, fb.TeamID, tokens)
	}
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	if tmr != nil && tmr.FailureCount > 0 {
		for _, f := range tmr.Errors {
			wasabee.Log.Debugf("[un]subscribe failed for %s: %s ; deleting token", tokens[f.Index], f.Reason)
			fb.Gid.FirebaseRemoveToken(tokens[f.Index])
		}
	}

	return nil
}
