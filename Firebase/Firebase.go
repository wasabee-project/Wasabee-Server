package wasabeefirebase

import (
	"context"
	"fmt"
	"time"

	firebase "firebase.google.com/go"
	// "firebase.google.com/go/messaging"

	"google.golang.org/api/option"

	"github.com/wasabee-project/Wasabee-Server"
)

// rate limit map
var rlmap map[wasabee.TeamID]rlt

type rlt struct {
	t time.Time
}

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

	rlmap = make(map[wasabee.TeamID]rlt)

	fbchan := wasabee.FirebaseInit()
	for fb := range fbchan {
		// wasabee.Log.Debugf("%s", fb.Cmd.String())
		switch fb.Cmd {
		case wasabee.FbccGenericMessage:
			_ = genericMessage(ctx, msg, fb)
		case wasabee.FbccAgentLocationChange:
			if rateLimit(fb.TeamID) {
				_ = agentLocationChange(ctx, msg, fb)
			}
		case wasabee.FbccMapChange:
			if rateLimit(fb.TeamID) {
				_ = mapChange(ctx, msg, fb)
			}
		case wasabee.FbccMarkerStatusChange:
			if rateLimit(fb.TeamID) {
				_ = markerStatusChange(ctx, msg, fb)
			}
		case wasabee.FbccMarkerAssignmentChange:
			_ = markerAssignmentChange(ctx, msg, fb)
		case wasabee.FbccLinkStatusChange:
			if rateLimit(fb.TeamID) {
				_ = linkStatusChange(ctx, msg, fb)
			}
		case wasabee.FbccLinkAssignmentChange:
			_ = linkAssignmentChange(ctx, msg, fb)
		case wasabee.FbccSubscribeTeam:
			_ = subscribeToTeam(ctx, msg, fb)
		case wasabee.FbccAgentLogin:
			_ = agentLogin(ctx, msg, fb)
		default:
			wasabee.Log.Debugf("unknown firebase command %d", fb.Cmd)
		}
	}
	return nil
}

func rateLimit(teamID wasabee.TeamID) bool {
	rl, ok := rlmap[teamID]
	now := time.Now()

	// first time sending to this team
	if !ok {
		rlmap[teamID] = rlt{
			t: now,
		}
		return true
	}

	waituntil := rl.t.Add(15 * time.Second)
	if now.Before(waituntil) {
		// wasabee.Log.Debugf("skipping firebase send to team %s", teamID)
		return false
	}

	rl.t = now
	rlmap[teamID] = rl
	return true
}
