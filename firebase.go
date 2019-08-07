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
		Log.Debug("shutting down firebase")
		fb.running = false
		close(fb.c)
	}
}

func (cc FirebaseCommandCode) String() string {
	return [...]string{"Quit", "Generic Message", "Agent Location Change", "Map Change", "Marker Status Change", "Marker Assignment Change", "Link Status Change", "Link Assignment Change", "Subscribe"}[cc]
}

// Functions called from Wasabee to message the firebase subsystem
func fbPush(fbc FirebaseCmd) {
	if !fb.running {
		Log.Debug("Firebase is not running, not sending msg")
		return
	}
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
			Msg:    tid.String(),
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
func (gid GoogleID) FirebaseTokens() ([]string, error) {
	var token string
	var toks []string

	rows, err := db.Query("SELECT token FROM firebase WHERE gid = ?", gid)
	if err != nil && err != sql.ErrNoRows {
		Log.Error(err)
		return toks, err
	}
	// this is technically redundant with the main return, but be explicit about what we want
	if err != nil && err == sql.ErrNoRows {
		return toks, nil
	}

	for rows.Next() {
		err := rows.Scan(&token)
		if err != nil {
			Log.Error(err)
			continue
		}
		toks = append(toks, token)
	}

	return toks, nil
}

// FirebaseInsertToken updates a token in the database for an agent
// gid is not unique, an agent may have any number of tokens (e.g. multiple devices/browsers) -- need a cleaning mechanism
func (gid GoogleID) FirebaseInsertToken(token string) error {
	_, err := db.Exec("INSERT INTO firebase (gid, token) VALUES (?, ?)", gid, token)
	if err != nil {
		Log.Error(err)
		return err
	}

	tl := gid.teamList()
	for _, teamID := range tl {
		gid.firebaseSubscribeTeam(teamID)
	}

	return nil
}

// Remove known token for a given user
func (gid GoogleID) FirebaseRemoveToken(token string) error {
	_, err := db.Exec("DELETE FROM firebase WHERE gid = ? AND token = ?", gid, token)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}
