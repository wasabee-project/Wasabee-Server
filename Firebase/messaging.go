package wasabeefirebase

import (
	"context"
	"firebase.google.com/go/messaging"
	"fmt"
	"github.com/wasabee-project/Wasabee-Server"
)

// SendMessage is registered with Wasabee for sending messages
func SendMessage(gid wasabee.GoogleID, message string) (bool, error) {
	gid.FirebaseGenericMessage(message)
	wasabee.Log.Debugw("generic message", "subsystem", "Firebase", "GID", gid)
	return false, nil
}

func genericMessage(ctx context.Context, c *messaging.Client, fb wasabee.FirebaseCmd) error {
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

		data := map[string]string{
			"msg": fb.Msg,
			"cmd": fb.Cmd.String(),
		}

		msg := messaging.Message{
			Token: token,
			Data:  data,
		}

		_, err = c.Send(ctx, &msg)
		if err != nil {
			wasabee.Log.Error(err)
			return err
		}
	}
	return nil
}

func agentLocationChange(ctx context.Context, c *messaging.Client, fb wasabee.FirebaseCmd) error {
	if fb.TeamID == "" {
		err := fmt.Errorf("only send location changes to teams")
		wasabee.Log.Error(err)
		return err
	}

	// webpush := webpushConfig()
	data := map[string]string{
		"gid": string(fb.Gid),
		"msg": fb.Msg,
		"cmd": fb.Cmd.String(),
	}

	msg := messaging.Message{
		Topic: string(fb.TeamID),
		Data:  data,
		// Webpush: &webpush,
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

	// webpush := webpushConfig()
	data := map[string]string{
		"opID":     string(fb.OpID),
		"markerID": fb.ObjID,
		"msg":      fb.Msg,
		"cmd":      fb.Cmd.String(),
	}
	msg := messaging.Message{
		Topic: string(fb.TeamID),
		Data:  data,
		// Webpush: &webpush,
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

		// webpush := webpushConfig()
		data := map[string]string{
			"opID":     string(fb.OpID),
			"markerID": fb.ObjID,
			"msg":      fb.Msg,
			"cmd":      fb.Cmd.String(),
		}

		msg := messaging.Message{
			Token: token,
			Data:  data,
			// Webpush: &webpush,
		}

		_, err = c.Send(ctx, &msg)
		if err != nil {
			wasabee.Log.Error(err)
			return err
		}
		// wasabee.Log.Debugf("sending marker assignment change to %s for op [%s] %s %s", fb.Gid, fb.OpID, fb.Msg, fb.Cmd.String());
	}
	return nil
}

func mapChange(ctx context.Context, c *messaging.Client, fb wasabee.FirebaseCmd) error {
	if fb.TeamID == "" {
		err := fmt.Errorf("only send status changes to teams")
		wasabee.Log.Error(err)
		return err
	}

	// webpush := webpushConfig()
	data := map[string]string{
		"opID": string(fb.OpID),
		"msg":  fb.Msg,
		"cmd":  fb.Cmd.String(),
	}

	msg := messaging.Message{
		Topic: string(fb.TeamID),
		Data:  data,
		// Webpush: &webpush,
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

	// webpush := webpushConfig()
	data := map[string]string{
		"opID":   string(fb.OpID),
		"linkID": fb.ObjID,
		"msg":    fb.Msg,
		"cmd":    fb.Cmd.String(),
	}
	msg := messaging.Message{
		Topic: string(fb.TeamID),
		Data:  data,
		// Webpush: &webpush,
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

		// notif := messaging.Notification{
		//	Title: "Link Assignment",
		//	Body:  "You have been assigned a new link",
		//}

		// webpush := webpushConfig()
		data := map[string]string{
			"opID":   string(fb.OpID),
			"linkID": fb.ObjID,
			"msg":    fb.Msg,
			"cmd":    fb.Cmd.String(),
		}

		msg := messaging.Message{
			Token: token,
			Data:  data,
			// Webpush: &webpush,
			// Notification: &notif,
		}

		_, err = c.Send(ctx, &msg)
		if err != nil {
			wasabee.Log.Error(err)
			return err
		}
	}
	return nil
}

func agentLogin(ctx context.Context, c *messaging.Client, fb wasabee.FirebaseCmd) error {
	if fb.TeamID == "" {
		err := fmt.Errorf("only send status changes to teams")
		wasabee.Log.Error(err)
		return err
	}

	data := map[string]string{
		"gid": string(fb.Gid),
		"msg": fb.Msg,
		"cmd": fb.Cmd.String(),
	}
	msg := messaging.Message{
		Topic: string(fb.TeamID),
		Data:  data,
		// Webpush: &webpush,
	}

	_, err := c.Send(ctx, &msg)
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	return nil
}

func broadcastDelete(ctx context.Context, c *messaging.Client, fb wasabee.FirebaseCmd) error {
	tokens, err := wasabee.FirebaseBroadcastList()
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	if len(tokens) == 0 {
		return nil
	}

	data := map[string]string{
		"msg":  fb.Msg,
		"cmd":  fb.Cmd.String(),
		"opID": string(fb.OpID),
	}

	// can send up to 500 per block
	for len(tokens) > 500 {
		subset := tokens[:500]
		tokens = tokens[500:]
		msg := messaging.MulticastMessage{
			Data:   data,
			Tokens: subset,
		}
		br, err := c.SendMulticast(ctx, &msg)
		if err != nil {
			wasabee.Log.Error(err)
			// carry on
		}
		wasabee.Log.Infow("delete broadcast block", "resource", string(fb.OpID), "success", br.SuccessCount, "failure", br.FailureCount)
		for pos, resp := range br.Responses {
			if !resp.Success {
				wasabee.Log.Infow("multicast error response", "token", subset[pos], "messageID", resp.MessageID, "error", resp.Error.Error())
				wasabee.FirebaseRemoveToken(tokens[pos])
			}
		}
	}

	msg := messaging.MulticastMessage{
		Data:   data,
		Tokens: tokens,
	}
	br, err := c.SendMulticast(ctx, &msg)
	if err != nil {
		wasabee.Log.Error(err)
		// carry on
	}
	wasabee.Log.Infow("delete broadcast final block", "resource", string(fb.OpID), "success", br.SuccessCount, "failure", br.FailureCount)
	for pos, resp := range br.Responses {
		if !resp.Success {
			wasabee.Log.Infow("multicast error response", "token", tokens[pos], "messageID", resp.MessageID, "error", resp.Error.Error())
			wasabee.FirebaseRemoveToken(tokens[pos])
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
	if len(tokens) == 0 {
		return nil
	}

	var tmr *messaging.TopicManagementResponse
	if fb.Msg == "subscribe" {
		tmr, err = c.SubscribeToTopic(ctx, tokens, string(fb.TeamID))
		// wasabee.Log.Debugf("subscribed %s to %s (%s)", fb.Gid, fb.TeamID, tokens)
	} else {
		tmr, err = c.UnsubscribeFromTopic(ctx, tokens, string(fb.TeamID))
		// wasabee.Log.Debugf("unsubscribed %s from %s (%s)", fb.Gid, fb.TeamID, tokens)
	}
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	if tmr != nil && tmr.FailureCount > 0 {
		for _, f := range tmr.Errors {
			wasabee.Log.Warnw("[un]subscribe failed; deleting token", "subsystem", "Firebase", "token", tokens[f.Index], "reason", f.Reason)
			fb.Gid.FirebaseRemoveToken(tokens[f.Index])
		}
	}
	return nil
}

/*
func webpushConfig() messaging.WebpushConfig {
	wpheaders := map[string]string{
		"TTL": "0",
	}
	wpnotif := messaging.WebpushNotification{
		Silent: true,
	}
	webpush := messaging.WebpushConfig{
		Headers:      wpheaders,
		Notification: &wpnotif,
	}
	return webpush
} */
