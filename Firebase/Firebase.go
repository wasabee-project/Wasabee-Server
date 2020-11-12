package wasabeefirebase

import (
	"context"
	"fmt"
	"sync"
	"time"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/messaging"
	"google.golang.org/api/option"
	"github.com/wasabee-project/Wasabee-Server"
)

var mux sync.Mutex

// rate limit map for map changes
var rlmap map[wasabee.TeamID]rlt

type rlt struct {
	t time.Time
	count uint32
}

var agentLocationMap map[wasabee.TeamID]alm

type alm struct {
	t time.Time
        g map[wasabee.GoogleID]time.Time
}

// ServeFirebase is the main startup function for the Firebase integration
func ServeFirebase(keypath string) error {
	wasabee.Log.Infow("startup", "subsystem", "Firebase", "version", firebase.Version, "message", "Firebase starting")

	ctx := context.Background()
	opt := option.WithCredentialsFile(keypath)
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		err := fmt.Errorf("error initializing firebase messaging: %v", err)
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
	agentLocationMap = make(map[wasabee.TeamID]alm)

	client, err := app.Auth(ctx)
	if err != nil {
		err := fmt.Errorf("error initializing firebase auth: %v", err)
		wasabee.Log.Error(err)
	}

	fbchan := wasabee.FirebaseInit(client)
	for fb := range fbchan {
		// wasabee.Log.Debug(fb.Cmd.String())
		switch fb.Cmd {
		case wasabee.FbccGenericMessage:
			_ = genericMessage(ctx, msg, fb)
		case wasabee.FbccTarget:
			_ = target(ctx, msg, fb)
		case wasabee.FbccAgentLocationChange:
			rateLimitAgentLocation(ctx, msg, fb)
		case wasabee.FbccMapChange:
			if rateLimitMapChange(fb.TeamID) {
				_ = mapChange(ctx, msg, fb)
			}
		case wasabee.FbccMarkerStatusChange:
			_ = markerStatusChange(ctx, msg, fb)
			// if rateLimitMapChange(fb.TeamID) { _ = markerStatusChange(ctx, msg, fb) }
		case wasabee.FbccMarkerAssignmentChange:
			_ = markerAssignmentChange(ctx, msg, fb)
		case wasabee.FbccLinkStatusChange:
			_ = linkStatusChange(ctx, msg, fb)
			// if rateLimitMapChange(fb.TeamID) { _ = linkStatusChange(ctx, msg, fb) }
		case wasabee.FbccLinkAssignmentChange:
			_ = linkAssignmentChange(ctx, msg, fb)
		case wasabee.FbccSubscribeTeam:
			_ = subscribeToTeam(ctx, msg, fb)
		case wasabee.FbccAgentLogin:
			_ = agentLogin(ctx, msg, fb)
		case wasabee.FbccBroadcastDelete:
			_ = broadcastDelete(ctx, msg, fb)
		case wasabee.FbccDeleteOp:
			_ = deleteOp(ctx, msg, fb)
		default:
			wasabee.Log.Warnw("unknown command", "subsystem", "Firebase", "command", fb.Cmd)
		}
	}
	return nil
}

// determines if clients should request individual agent or full team (if the client supports it)
func rateLimitAgentLocation(ctx context.Context, msg *messaging.Client, fb wasabee.FirebaseCmd) {
	// wasabee.Log.Debug(fb)
	now := time.Now()

	mux.Lock()
	defer mux.Unlock()
	rl, ok := agentLocationMap[fb.TeamID]

	// first time sending to this team
	if !ok {
		// wasabee.Log.Debugw("first rate limited agent location request for team", "resource", fb.TeamID)
		rl = alm{
			t: now,
			g: make(map[wasabee.GoogleID]time.Time),
		}
		rl.g[fb.Gid] = now
		agentLocationMap[fb.TeamID] = rl
		_ = agentLocationChange(ctx, msg, fb)
		return
	}

	waituntil := rl.t.Add(10 * time.Second)
	if now.Before(waituntil) {
		rl.g[fb.Gid] = now
		agentLocationMap[fb.TeamID] = rl
		// wasabee.Log.Debugw("skipping agent location firebase send to team", "subsystem", "Firebase", "firebase command", fb)
		// add to a queue to send in N seconds
		return
	}

	// long enough since previous message to this team
	rl.t = now
	agentsThrottled := len(rl.g)

	// reset the map and add an entry 
	rl.g = make(map[wasabee.GoogleID]time.Time)
	rl.g[fb.Gid] = now
	agentLocationMap[fb.TeamID] = rl

	if agentsThrottled > 1 {
		// more than one agent throttled, instruct the clients to fetch whole team
		fb.Gid = ""
		// wasabee.Log.Debugw("whole-team agent location firebase send to team", "subsystem", "Firebase", "firebase command", fb)
	} else {
		// include GID so client can pull the individual agent directly
		// wasabee.Log.Debugw("single agent location firebase send to team", "subsystem", "Firebase", "firebase command", fb)
	}
	_ = agentLocationChange(ctx, msg, fb)
}

func rateLimitMapChange(teamID wasabee.TeamID) bool {
	now := time.Now()

	mux.Lock()
	defer mux.Unlock()
	rl, ok := rlmap[teamID]

	// first time sending to this team
	if !ok {
		rlmap[teamID] = rlt{
			t: now,
			count: 0,
		}
		return true
	}

	waituntil := rl.t.Add(3 * time.Second)
	if now.Before(waituntil) {
		rl.count++
		// wasabee.Log.Debugw("skipping map change firebase send to team", "subsystem", "Firebase", "resource", teamID, "skip count", rl.count)
		// add to a queue to send in 3 seconds
		return false
	}

	rl.t = now
	rl.count = 0
	rlmap[teamID] = rl
	return true
}
