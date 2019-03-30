package PhDevBin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
	// "github.com/cloudkucooland/PhDevBin/Messaging"
)

// WaypointCommand is defined by the OwnTracks JSON format.
// It is the top level item in the JSON file when the OwnTracks app sends any waypoint changes
type WaypointCommand struct {
	Type      string        `json:"_type"`
	Action    string        `json:"action"`
	Waypoints WaypointsList `json:"waypoints"`
}

// WaypointsList is defined by the OwnTracks JSON format.
// It is always encapsulated in a WaypointCommand and aways contains a list of waypoints.
type WaypointsList struct {
	Waypoints []Waypoint `json:"waypoints"`
	Type      string     `json:"_type"`
}

// Waypoint is defined by the OwnTracks JSON format.
// It is the datatype which contains the information about a waypoint.
type Waypoint struct {
	Type       string  `json:"_type"`
	Desc       string  `json:"desc"`
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	Radius     float64 `json:"rad"`
	ID         float64 `json:"tst"`
	UUID       string  `json:"uuid,omitempty"`
	Major      string  `json:"major,omitempty"`
	Minor      string  `json:"minor,omitempty"`
	Share      bool    `json:"share"`                // this was removed from the API, but I'm going to leave it for now
	MarkerType string  `json:"markertype,omitempty"` // PhDevBin extension
	TeamID     string  `json:"teamid,omitempty"`     // PhDevBin extension
	Distance   float64 `json:"distance,omitempty"`   // PhDevBin extension
}

// Location is defined by the OwnTracks JSON format.
// This is what is sent from and to the OwnTracks app to indicate a person's location.
// Type, Lat, Lon, and ShortName are required, all others are optional.
// N.B. InRegions is not documented but is sent by the iOS client.
type Location struct {
	Type      string   `json:"_type"`
	Lat       float64  `json:"lat"`
	Lon       float64  `json:"lon"`
	Topic     string   `json:"topic,omitempty"`
	ShortName string   `json:"tid"`
	T         string   `json:"t,omitempty"`
	Conn      string   `json:"conn,omitempty"`
	Altitude  float64  `json:"alt,omitempty"`
	Battery   float64  `json:"batt,omitempty"`
	Accuracy  float64  `json:"acc,omitempty"`
	Vac       float64  `json:"vac,omitempty"`
	TimeStamp float64  `json:"tst,omitempty"`
	Velocity  float64  `json:"vel,omitempty"`
	InRegions []string `json:"inregions,omitempty"`
}

// Transition is defined by the OwnTracks JSON format.
// It is sent when a person enters or leaves a defined Waypoint's radius.
type Transition struct {
	Type      string  `json:"_type"`
	Event     string  `json:"event"`
	ID        float64 `json:"wtst"`
	TimeStamp float64 `json:"tst"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Topic     string  `json:"topic"`
	Trigger   string  `json:"t"`
	Accuracy  float64 `json:"acc"`
	Tid       string  `json:"tid"`
	Desc      string  `json:"desc"`
}

// OwnTracksUpdate simply stores incoming OwnTracks data into the database
func (gid GoogleID) OwnTracksUpdate(otdata json.RawMessage, lat, lon float64) error {
	clean, _ := gid.ownTracksTidy(string(otdata))
	_, err := db.Exec("UPDATE otdata SET otdata = ? WHERE gid = ?", string(clean), gid)
	if err != nil {
		Log.Notice(err)
	}
	err = gid.UserLocation(strconv.FormatFloat(lat, 'f', -1, 64), strconv.FormatFloat(lon, 'f', -1, 64), "OwnTracks")
	return err
}

// OwnTracksTeams returns a JSON message containing all the agents who are members of the same teams as the requested agent (gid)
// It also includes all WayPoints for these teams.
// This is sufficient for returning directly to the OwnTracks app
func (gid GoogleID) OwnTracksTeams() (json.RawMessage, error) {
	var locs []json.RawMessage
	var tmp sql.NullString

	r, err := db.Query("SELECT DISTINCT o.otdata FROM otdata=o, userteams=ut, locations=l WHERE o.gid = ut.gid AND o.gid != ? AND ut.teamID IN (SELECT teamID FROM userteams WHERE gid = ? AND state != 'Off') AND ut.state != 'Off' AND o.gid = l.gid AND l.upTime > SUBTIME(NOW(), '12:00:00')", gid, gid)
	if err != nil {
		Log.Error(err)
		return json.RawMessage(""), err
	}
	defer r.Close()
	for r.Next() {
		err := r.Scan(&tmp)
		if err != nil {
			Log.Error(err)
			return json.RawMessage(""), err
		}
		if tmp.Valid && tmp.String != "{ }" {
			// clean, _ := ownTracksTidy(gid, tmp.String)
			// locs = append(locs, clean)
			locs = append(locs, json.RawMessage(tmp.String))
		}
	}
	s, _ := json.Marshal(locs)

	var wp WaypointCommand
	err = gid.otWaypoints(&wp)
	if err != nil {
		Log.Error(err)
		return s, err
	}
	wps, _ := json.Marshal(wp)
	locs = append(locs, wps)

	s, _ = json.Marshal(locs)
	// Log.Debug(string(s))
	return s, nil
}

// OwnTracksWaypoints returns a JSON formatted list of waypoints.
// iOS does not support sending locations and Waypoints in the same packet.
func (gid GoogleID) OwnTracksWaypoints() (json.RawMessage, error) {
	j := json.RawMessage("{ }")

	var wp WaypointCommand
	err := gid.otWaypoints(&wp)
	if err != nil {
		Log.Error(err)
		return j, err
	}

	j, err = json.Marshal(wp)
	if err != nil {
		Log.Error(err)
		return j, err
	}
	return j, nil
}

func (gid GoogleID) otWaypoints(wp *WaypointCommand) error {
	wp.Type = "cmd"
	wp.Action = "setWaypoints"
	wp.Waypoints.Type = "waypoints"

	var Id, lat, lon, radius sql.NullString
	var tmpWaypoint Waypoint
	tmpWaypoint.Type = "waypoint"

	wr, err := db.Query("SELECT Id, w.teamID, Y(loc) as lat, X(loc) as lon, radius, type, name FROM waypoints=w, userteams=ut WHERE ut.teamID = w.teamID AND ut.teamID IN (SELECT teamID FROM userteams WHERE ut.gid = ? AND ut.state != 'Off')", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer wr.Close()
	for wr.Next() {
		err := wr.Scan(&Id, &tmpWaypoint.TeamID, &lat, &lon, &radius, &tmpWaypoint.MarkerType, &tmpWaypoint.Desc)
		if err != nil {
			Log.Error(err)
			return nil
		}
		if Id.Valid {
			f, _ := strconv.ParseFloat(Id.String, 64)
			tmpWaypoint.ID = f
		}
		if lat.Valid {
			f, _ := strconv.ParseFloat(lat.String, 64)
			tmpWaypoint.Lat = f
		}
		if lon.Valid {
			f, _ := strconv.ParseFloat(lon.String, 64)
			tmpWaypoint.Lon = f
		}
		if radius.Valid {
			f, _ := strconv.ParseFloat(radius.String, 64)
			tmpWaypoint.Radius = f
		}
		tmpWaypoint.Share = true
		wp.Waypoints.Waypoints = append(wp.Waypoints.Waypoints, tmpWaypoint)
	}

	/* mr, err := db.Query("SELECT Id, w.teamID, Y(loc) as lat, X(loc) as lon, radius, type, name FROM waypoints=t, userteams=ut WHERE ut.teamID = w.teamID AND ut.teamID IN (SELECT teamID FROM userteams WHERE ut.gid = ? AND ut.state != 'Off')", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer mr.Close() */
	return nil
}

// OwnTracksTransition is called when an agent enters or leaves a WayPoint's radius
// currently a stub which only sends a message alerting the user that they have made the transition
// future features are still being considered
func (gid GoogleID) OwnTracksTransition(transition json.RawMessage) (json.RawMessage, error) {
	var t Transition
	j := json.RawMessage("{ }")

	if err := json.Unmarshal(transition, &t); err != nil {
		Log.Notice(err)
		return j, err
	}

	// XXX do something here -- or not
	Log.Debugf("%s transition %s: %s (%f)", gid, t.Event, t.Desc, t.ID)
	gid.SendMessage(fmt.Sprintf("%s area: %s", t.Event, t.Desc))

	return j, nil
}

// ownTracksTidy parses OwnTracks data (JSON format) and returns a version of that data
// which has been cleaned and formatted for consistency
// future features for this are still being considered
// Should this be a method instead of a function? On what datatype?
func (gid GoogleID) ownTracksTidy(otdata string) (json.RawMessage, error) {
	var l Location
	if err := json.Unmarshal(json.RawMessage(otdata), &l); err != nil {
		Log.Notice(err)
		return json.RawMessage(otdata), err
	}

	// rewrite topic to be owntracks/agent/device-id ?

	redo, err := json.Marshal(l)
	if err != nil {
		Log.Notice(err)
		return json.RawMessage(otdata), err
	}

	return redo, nil
}

// ownTracksExternalUpdate is called when an agent's location is set through other means
// such as via the web or telegram interface. This allows agents to choose the method of
// location reporting which suits their needs best.
func (gid GoogleID) ownTracksExternalUpdate(lat, lon, source string) error {
	var otdata string
	err := db.QueryRow("SELECT otdata FROM otdata WHERE gid = ?", gid).Scan(&otdata)
	if err != nil {
		Log.Error(err)
		return err
	}

	var l Location
	if err := json.Unmarshal(json.RawMessage(otdata), &l); err != nil {
		Log.Notice(err)
		return err
	}

	l.Type = "location"
	if l.ShortName == "" {
		l.ShortName = "XX"
	}
	l.Lat, _ = strconv.ParseFloat(lat, 64)
	l.Lon, _ = strconv.ParseFloat(lon, 64)
	l.Topic = fmt.Sprintf("owntracks/%s/%s", gid, source)

	t := time.Now()
	l.TimeStamp = float64(t.Unix())

	redo, err := json.Marshal(l)
	if err != nil {
		Log.Notice(err)
		return err
	}

	_, err = db.Exec("UPDATE otdata SET otdata = ? WHERE gid = ?", redo, gid)
	if err != nil {
		Log.Notice(err)
		return err
	}
	return nil
}

// OwnTracksSetWaypoint is called when a waypoint message is received from the OwnTracks application
func (gid GoogleID) OwnTracksSetWaypoint(wp json.RawMessage) (json.RawMessage, error) {
	// Log.Debug(string(wp))
	var w Waypoint
	j, _ := gid.OwnTracksWaypoints()

	team, err := gid.PrimaryTeam() // cache this...
	if err != nil || team == "" {
		e := errors.New("Unable to determine primary team for SetWaypoint")
		Log.Notice(e)
		return j, e
	}

	if err := json.Unmarshal(wp, &w); err != nil {
		// Log.Notice(err)
		return j, err
	}

	if err = ownTracksWriteWaypoint(w, team); err != nil {
		// Log.Notice(err)
		return j, err
	}

	j, err = gid.OwnTracksWaypoints()
	return j, err
}

// ownTracksWriteWaypoint is called from SetWaypoint and SetWaypointList and writes the data to the database.
// XXX need to write a thread to invalidate expired waypoints and then remove them
func ownTracksWriteWaypoint(w Waypoint, team string) error {
	_, err := db.Exec("INSERT INTO waypoints VALUES (?,?,POINT(?, ?),?,?,?,FROM_UNIXTIME(? + (86400 * 14))) ON DUPLICATE KEY UPDATE Id = ?, loc = POINT(?, ?), radius = ?, name = ?",
		w.ID, team, w.Lon, w.Lat, w.Radius, "target", w.Desc, w.ID,
		w.ID, w.Lon, w.Lat, w.Radius, w.Desc)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

// OwnTracksSetWaypointList is called when a waypoint list is received from the OwnTracks application
func (gid GoogleID) OwnTracksSetWaypointList(wp json.RawMessage) (json.RawMessage, error) {
	// Log.Debug(string(wp))
	var w WaypointsList
	j, _ := gid.OwnTracksWaypoints()

	team, err := gid.PrimaryTeam()
	if err != nil || team == "" {
		e := errors.New("Unable to determine primary team for SetWaypointList")
		Log.Notice(e)
		return j, e
	}

	if err := json.Unmarshal(wp, &w); err != nil {
		Log.Notice(err)
		return j, err
	}

	for _, waypoint := range w.Waypoints {
		if err := ownTracksWriteWaypoint(waypoint, team); err != nil {
			Log.Notice(err)
			return j, err
		}
	}

	j, err = gid.OwnTracksWaypoints()
	return j, err
}

// PrimaryTeam is called to determine an agent's primary team -- which is where Waypoint data is saved
// move to model_team.go
func (gid GoogleID) PrimaryTeam() (string, error) {
	var primary string
	err := db.QueryRow("SELECT teamID FROM userteams WHERE gid = ? AND state = 'Primary'", gid).Scan(&primary)
	if err != nil && err.Error() == "sql: no rows in result set" {
		Log.Debug("Primary Team Not Set")
		return "", nil
	}
	if err != nil {
		Log.Error(err)
		return "", err
	}
	return primary, nil
}
