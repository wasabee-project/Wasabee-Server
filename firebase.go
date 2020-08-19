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
	FbccAgentLogin
	FbccBroadcastDelete
	FbccDeleteOp
	FbccTarget
)

// String is the string value of the Firebase Command Code
// yes, delete is the same for broadcast and direct
func (cc FirebaseCommandCode) String() string {
	return [...]string{"Quit", "Generic Message", "Agent Location Change", "Map Change", "Marker Status Change", "Marker Assignment Change", "Link Status Change", "Link Assignment Change", "Subscribe", "Login", "Delete", "Delete", "Target"}[cc]
}

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
		Log.Infow("shutdown", "message", "shutting down firebase")
		fb.running = false
		close(fb.c)
	}
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
// this is not really true, only those with the server key can adjust topic membership, so it would be safe to share location directly
// but this is probably sufficient and has worked well so far
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

// FirebaseGenericMessage sends a free-form message to a single agent
func (gid GoogleID) FirebaseGenericMessage(msg string) {
	if !fb.running {
		return
	}

	fbPush(FirebaseCmd{
		Cmd: FbccGenericMessage,
		Gid: gid,
		Msg: msg,
	})
}

// FirebaseTarget sends a JSON formatted target to the agent
func (gid GoogleID) FirebaseTarget(msg string) {
	if !fb.running {
		return
	}

	fbPush(FirebaseCmd{
		Cmd: FbccTarget,
		Gid: gid,
		Msg: msg,
	})
}

// FirebaseTarget sends a JSON formatted target to a team
func (teamID TeamID) FirebaseTarget(msg string) {
	if !fb.running {
		return
	}

	fbPush(FirebaseCmd{
		Cmd: FbccTarget,
		TeamID: teamID,
		Msg: msg,
	})
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
func (o *Operation) firebaseMarkerStatus(markerID MarkerID, status string) {
	if !fb.running {
		return
	}

	if len(o.Teams) == 0 {
		_ = o.PopulateTeams()
	}
	for _, t := range o.Teams {
		fbPush(FirebaseCmd{
			Cmd:    FbccMarkerStatusChange,
			TeamID: t.TeamID,
			OpID:   o.ID,
			ObjID:  string(markerID),
			Msg:    status,
		})
	}
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

func (o *Operation) firebaseLinkStatus(linkID LinkID, completed bool) {
	if !fb.running {
		return
	}

	msg := "complete"
	if !completed {
		msg = "incomplete"
	}

	if len(o.Teams) == 0 {
		_ = o.PopulateTeams()
	}
	for _, t := range o.Teams {
		fbPush(FirebaseCmd{
			Cmd:    FbccLinkStatusChange,
			TeamID: t.TeamID,
			OpID:   o.ID,
			ObjID:  string(linkID),
			Msg:    msg,
		})
	}
}

func (o *Operation) firebaseMapChange() {
	if !fb.running {
		return
	}

	if len(o.Teams) == 0 {
		_ = o.PopulateTeams()
	}
	for _, t := range o.Teams {
		fbPush(FirebaseCmd{
			Cmd:    FbccMapChange,
			TeamID: t.TeamID,
			OpID:   o.ID,
			Msg:    "changed",
		})
	}
	// Log.Debugw("sending mapchange via firebase", "subsystem", "Firebase", "resource", o.ID)
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

// FirebaseAgentLogin sends a notification to teammates when an agent logs in
func (gid GoogleID) FirebaseAgentLogin() {
	if !fb.running {
		return
	}

	tl := gid.teamList()
	for _, teamID := range tl {
		fbPush(FirebaseCmd{
			Cmd:    FbccAgentLogin,
			Gid:    gid,
			TeamID: teamID,
			Msg:    "Login",
		})
	}
}

func firebaseBroadcastDelete(opID OperationID) {
	if !fb.running {
		return
	}

	fbPush(FirebaseCmd{
		Cmd:  FbccBroadcastDelete,
		OpID: opID,
		Msg:  "Delete",
	})
}

// FirebaseDeleteOp instructs a single agent to delete a specified op
func (gid GoogleID) FirebaseDeleteOp(opID OperationID) {
	if !fb.running {
		return
	}

	fbPush(FirebaseCmd{
		Cmd:  FbccDeleteOp,
		Gid:  gid,
		OpID: opID,
		Msg:  "Delete",
	})
}

// Functions called from Firebase to use Wasabee resources

// FirebaseTokens gets an agents FirebaseToken from the database
// token may be "" if it has not been set for a agent
func (gid GoogleID) FirebaseTokens() ([]string, error) {
	var token string
	var toks []string

	rows, err := db.Query("SELECT DISTINCT token FROM firebase WHERE gid = ?", gid)
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

// FirebaseInsertToken adds a token in the database for an agent.
// gid is not unique, an agent may have any number of tokens (e.g. multiple devices/browsers).
// Pruning of dead tokens takes place in the senders upon error.
func (gid GoogleID) FirebaseInsertToken(token string) error {
	var count int
	err := db.QueryRow("SELECT COUNT(gid) FROM firebase WHERE token = ? AND gid = ?", token, gid).Scan(&count)
	if err != nil {
		Log.Error(err)
		return err
	}

	if count > 0 {
		return nil
	}

	Log.Debugw("adding token", "subsystem", "Firebase", "GID", gid, "token", token)
	_, err = db.Exec("INSERT INTO firebase (gid, token) VALUES (?, ?)", gid, token)
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

// FirebaseRemoveToken removes a known token for a given agent
func (gid GoogleID) FirebaseRemoveToken(token string) {
	_, err := db.Exec("DELETE FROM firebase WHERE gid = ? AND token = ?", gid, token)
	if err != nil {
		Log.Error(err)
	}
}

// FirebaseRemoveAllTokens removes all tokens for a given agent
func (gid GoogleID) FirebaseRemoveAllTokens() {
	_, err := db.Exec("DELETE FROM firebase WHERE gid = ?", gid)
	if err != nil {
		Log.Error(err)
	}
}

// FirebaseRemoveToken removes known token regardless of the agent
func FirebaseRemoveToken(token string) {
	_, err := db.Exec("DELETE FROM firebase WHERE token = ?", token)
	if err != nil {
		Log.Error(err)
	}
}

// FirebaseBroadcastList returns all known firebase tokens for messaging all agents
// Firebase Multicast messages are limited to 500 tokens each, the caller must
// break the list up if necessary.
func FirebaseBroadcastList() ([]string, error) {
	var out []string

	rows, err := db.Query("SELECT DISTINCT token FROM firebase")
	if err != nil && err == sql.ErrNoRows {
		return out, nil
	}
	if err != nil {
		Log.Error(err)
		return out, err
	}

	var token string
	for rows.Next() {
		err := rows.Scan(&token)
		if err != nil {
			Log.Error(err)
			continue
		}
		out = append(out, token)
	}
	return out, nil
}
