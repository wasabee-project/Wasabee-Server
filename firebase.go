package wasabee

import (
	"database/sql"
)

var fb struct {
	running bool
	c       chan FirebaseCmd
}

// FirebaseCommandCode is the command codes used for communicating with the Firebase module
type FirebaseCommandCode int

// FbccQuit et al. are the friendly names for the FirebaseCommandCode
const (
	FbccQuit FirebaseCommandCode = iota
	FbccGenericMessage
	FbccAgentLocationChange
	FbccMapChange
	FbccMarkerStatusChange
	FbccMarkerAssignmentChange
	FbccLinkStatusChange
	FbccLinkAssignmentChange
	FbccSubscribeTeam
)

// FirebaseCmd is the struct passed to the Firebase module to take actions -- required params depend on the FBCC
type FirebaseCmd struct {
	Cmd    FirebaseCommandCode
	TeamID TeamID
	OpID   OperationID
	ObjID  string // either LinkID, MarkerID ... XXX define ObjectID type?
	Gid    GoogleID
	Msg    string
}

// FirebaseInit creates the channel used to pass messages to the Firebase subsystem
func FirebaseInit() <-chan FirebaseCmd {
	out := make(chan FirebaseCmd, 3)

	fb.c = out
	fb.running = true
	return out
}

// FirebaseClose shuts down the channel when done
func FirebaseClose() {
	if fb.running {
		fb.running = false
		close(fb.c)
	}
}

func (cc FirebaseCommandCode) String() string {
	return [...]string{"Quit", "Generic Message", "Agent Location Change", "Map Change", "Marker Status Change", "Marker Assignment Change", "Link Status Change", "Link Assignment Change"}[cc]
}

// Functions called from Wasabee to message the firebase subsystem
// func fbPush(cc FirebaseCommandCode, teamID TeamID, opID OperationID, objID string, gid GoogleID, msg string) {
func fbPush(fbc FirebaseCmd) {
	if !fb.running {
		Log.Debug("Firebase is not running, not sending msg")
		return
	}
	// Log.Debugf("sending %s", fbc.Cmd.String())
	// XXX other sanity checking here?
	fb.c <- fbc
}

// notifiy all subscribers to the agent's teams that they have moved
// do not share the location since it is possible to subscribe to firebase topics without being on the team
func (gid GoogleID) firebaseAgentLocation() {
	if !fb.running {
		return
	}

	for _, tid := range gid.teamList() {
		fbPush(FirebaseCmd{
			Cmd:    FbccAgentLocationChange,
			TeamID: tid,
			Gid:    gid,
		})
	}
}

// notifiy the agent that they have a new assigned marker in a given op
func (opID OperationID) firebaseAssignMarker(gid GoogleID, markerID MarkerID) {
	if !fb.running {
		return
	}

	fbPush(FirebaseCmd{
		Cmd:   FbccMarkerAssignmentChange,
		OpID:  opID,
		ObjID: string(markerID),
		Gid:   gid,
		Msg:   "assigned",
	})
}

// notify a team that a marker's status has changed
func (opID OperationID) firebaseMarkerStatus(markerID MarkerID, status string) {
	if !fb.running {
		return
	}

	teamID, err := opID.GetTeamID()
	if err != nil {
		Log.Error(err)
		return
	}
	fbPush(FirebaseCmd{
		Cmd:    FbccMarkerStatusChange,
		TeamID: teamID,
		OpID:   opID,
		ObjID:  string(markerID),
		Msg:    status,
	})
}

// notifiy the agent that they have a new assigned marker in a given op
func (opID OperationID) firebaseAssignLink(gid GoogleID, linkID LinkID) {
	if !fb.running {
		return
	}

	fbPush(FirebaseCmd{
		Cmd:   FbccLinkAssignmentChange,
		OpID:  opID,
		ObjID: string(linkID),
		Gid:   gid,
		Msg:   "assigned",
	})
}

func (opID OperationID) firebaseLinkStatus(linkID LinkID, completed bool) {
	if !fb.running {
		return
	}

	msg := "complete"
	if !completed {
		msg = "incomplete"
	}

	teamID, err := opID.GetTeamID()
	if err != nil {
		Log.Error(err)
		return
	}
	fbPush(FirebaseCmd{
		Cmd:    FbccLinkStatusChange,
		TeamID: teamID,
		OpID:   opID,
		ObjID:  string(linkID),
		Msg:    msg,
	})
}

func (opID OperationID) firebaseMapChange() {
	if !fb.running {
		return
	}

	teamID, err := opID.GetTeamID()
	if err != nil {
		Log.Error(err)
		return
	}
	fbPush(FirebaseCmd{
		Cmd:    FbccMapChange,
		TeamID: teamID,
		OpID:   opID,
		Msg:    "changed",
	})
}

func (gid GoogleID) firebaseSubscribeTeam(teamID TeamID) {
	if !fb.running {
		return
	}

	fbPush(FirebaseCmd{
		Cmd:    FbccSubscribeTeam,
		Gid:    gid,
		TeamID: teamID,
		Msg:    "subscribe",
	})
}

func (gid GoogleID) firebaseUnsubscribeTeam(teamID TeamID) {
	if !fb.running {
		return
	}

	fbPush(FirebaseCmd{
		Cmd:    FbccSubscribeTeam,
		Gid:    gid,
		TeamID: teamID,
		Msg:    "unsubscribe",
	})
}

// Functions called from Firebase to use Wasabee resources

// FirebaseToken gets an agents FirebaseToken from the database
// token may be "" if it has not been set for a user
func (gid GoogleID) FirebaseToken() (string, error) {
	var token string
	err := db.QueryRow("SELECT token FROM firebase WHERE gid = ?", gid).Scan(&token)
	if err != nil && err == sql.ErrNoRows {
		Log.Error(err)
		return "", err
	}

	return token, nil
}

// FirebaseInsertToken updates a token in the database for an agent
func (gid GoogleID) FirebaseInsertToken(token string) error {
	_, err := db.Exec("INSERT INTO firebase (gid, token) VALUES (?, ?) ON DUPLICATE UPDATE token = ?", gid, token, token)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}
