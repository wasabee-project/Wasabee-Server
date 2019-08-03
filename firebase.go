package wasabee

import (
	"database/sql"
)

var fb struct {
	running bool
	c       chan FirebaseCmd
}

type FirebaseCommandCode int

const (
	FbccQuit FirebaseCommandCode = iota
	FbccGenericMessage
	FbccAgentLocationChange
	FbccMapChange
	FbccMarkerStatusChange
	FbccMarkerAssignmentChange
	FbccLinkStatusChange
	FbccLinkAssignmentChange
)

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
func firebaseGeneric(cc FirebaseCommandCode, teamID TeamID, opID OperationID, objID string, gid GoogleID, msg string) {
	if !fb.running {
		Log.Debug("Firebase is not running, not sending msg")
		return
	}

	var fbc = FirebaseCmd{
		Cmd:    cc,
		TeamID: teamID,
		OpID:   opID,
		ObjID:  objID,
		Gid:    gid,
		Msg:    msg,
	}
	fb.c <- fbc
}

func (gid GoogleID) firebaseAgentLocation() {
	if !fb.running {
		return
	}

	// notify every team the agent has enabled
	// do not share the location since it is possible to subscribe to firebase topics without being on the team
	// XXX it is possible to send to a list of topics (teams) -- save on multiple sends to google...
	for _, t := range gid.teamList() {
		firebaseGeneric(FbccAgentLocationChange, t, "", "", gid, "")
	}
}

func (opID OperationID) firebaseAssignMarker(gid GoogleID, markerID MarkerID) {
	if !fb.running {
		return
	}

	firebaseGeneric(FbccMarkerAssignmentChange, "", opID, string(markerID), gid, "assigned")
}

func (opID OperationID) firebaseMarkerStatus(markerID MarkerID, status string) {
	if !fb.running {
		return
	}

	teamID, err := opID.GetTeamID()
	if err != nil {
		Log.Error(err)
		return
	}
	firebaseGeneric(FbccMarkerStatusChange, teamID, opID, string(markerID), "", status)
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
