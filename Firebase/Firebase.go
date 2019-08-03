package wasabeefirebase

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go"
	// "firebase.google.com/go/messaging"

	"google.golang.org/api/option"

	"github.com/wasabee-project/Wasabee-Server"
)

// ServeFirebase is the main startup function for the Firebase integration
func ServeFirebase(keypath string) error {
	wasabee.Log.Debug("starting Firebase")

	ctx := context.Background()
	opt := option.WithCredentialsFile(keypath)
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		err := fmt.Errorf("error initializing firebase: %v", err)
		wasabee.Log.Error(err)
		return err
	}

	// make sure we can send messages
	msg, err := app.Messaging(ctx)
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}

	fbchan := wasabee.FirebaseInit()
	for fb := range fbchan {
		// wasabee.Log.Debugf("processing %s", fb.Cmd.String())
		switch fb.Cmd {
		case wasabee.FbccAgentLocationChange:
			_ = agentLocationChange(ctx, msg, fb)
		case wasabee.FbccMapChange:
			_ = mapChange(ctx, msg, fb)
		case wasabee.FbccMarkerStatusChange:
			_ = markerStatusChange(ctx, msg, fb)
		case wasabee.FbccMarkerAssignmentChange:
			_ = markerAssignmentChange(ctx, msg, fb)
		case wasabee.FbccLinkStatusChange:
			_ = linkStatusChange(ctx, msg, fb)
		case wasabee.FbccLinkAssignmentChange:
			_ = linkAssignmentChange(ctx, msg, fb)
		case wasabee.FbccSubscribeTeam:
			_ = subscribeToTeam(ctx, msg, fb)
		default:
			wasabee.Log.Debugf("Unknown Firebase command %d", fb.Cmd)
		}
	}
	return nil
}
