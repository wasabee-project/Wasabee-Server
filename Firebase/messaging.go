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

	token, err := fb.Gid.FirebaseToken()
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	if token == "" {
		return nil
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

	token, err := fb.Gid.FirebaseToken()
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	if token == "" {
		return nil
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
	return nil
}

func subscribeToTeam(ctx context.Context, c *messaging.Client, fb wasabee.FirebaseCmd) error {
	if fb.Gid == "" {
		return nil
	}
	if fb.TeamID == "" {
		return nil
	}

	token, err := fb.Gid.FirebaseToken()
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	if token == "" {
		return nil
	}

	t := []string{token}
	if fb.Msg == "subscribe" {
		_, err = c.SubscribeToTopic(ctx, t, string(fb.TeamID))
		wasabee.Log.Debugf("subscribed %s to %s", fb.Gid, fb.TeamID)
	} else {
		_, err = c.UnsubscribeFromTopic(ctx, t, string(fb.TeamID))
		wasabee.Log.Debugf("unsubscribed %s from %s", fb.Gid, fb.TeamID)
	}
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	return nil
}
