package PhDevBin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"fmt"
	"time"
)

// a command to set waypoints
type WaypointCommand struct {
	Type      string        `json:"_type"`
	Action    string        `json:"action"`
	Waypoints WaypointsList `json:"waypoints"`
}

// a list of waypoints
type WaypointsList struct {
	Waypoints []Waypoint `json:"waypoints"`
	Type      string     `json:"_type"`
}

// individual waypoints
type Waypoint struct {
	Type   string  `json:"_type"`
	Desc   string  `json:"desc"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	Radius float64 `json:"rad"`
	ID     float64 `json:"tst"`
	UUID   string  `json:"uuid,omitempty"`
	Major  string  `json:"major,omitempty"`
	Minor  string  `json:"minor,omitempty"`
	Share  bool    `json:"share"` // this was removed from the API, but I'm going to leave it for now
}

// location
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

// every query in here should be prepared since these are called VERY frequently
// data is vague on prepared statement performance inprovement -- do real testing
func OwnTracksUpdate(gid string, otdata json.RawMessage, lat, lon float64) error {
	clean, _ := ownTracksTidy(gid, string(otdata))
	_, err := db.Exec("UPDATE otdata SET otdata = ? WHERE gid = ?", string(clean), gid)
	if err != nil {
		Log.Notice(err)
	}
	err = UserLocation(gid, strconv.FormatFloat(lat, 'f', -1, 64), strconv.FormatFloat(lon, 'f', -1, 64), "OwnTracks")
	return err
}

func OwnTracksTeams(gid string) (json.RawMessage, error) {
	var locs []json.RawMessage
	var tmp sql.NullString

	r, err := db.Query("SELECT DISTINCT o.otdata FROM otdata=o, userteams=ut, locations=l WHERE o.gid = ut.gid AND o.gid != ? AND ut.teamID IN (SELECT teamID FROM userteams WHERE gid = ? AND state != 'Off') AND ut.state != 'Off' AND l.upTime > SUBTIME(NOW(), '12:00:00')", gid, gid)
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
	wp.Type = "cmd"
	wp.Action = "setWaypoints"
	wp.Waypoints.Type = "waypoints"
	// not all of these can be null, just write to the struct directly
	// yeah, but lots of ParseFloat conversions in here... maybe a sexier way, but this is readable
	var Id, teamID, lat, lon, radius, typeId, nameId sql.NullString
	var tmpTarget Waypoint
	tmpTarget.Type = "waypoint"

	wr, err := db.Query("SELECT Id, t.teamID, X(loc) as lat, Y(loc) as lon, radius, type, name FROM target=t, userteams=ut WHERE ut.teamID = t.teamID AND ut.teamID IN (SELECT teamID FROM userteams WHERE ut.gid = ? AND ut.state != 'Off')", gid)
	if err != nil {
		Log.Error(err)
		return s, nil // a lie, but getting people location and no targets is better than no data
	}
	defer wr.Close()
	for wr.Next() {
		err := wr.Scan(&Id, &teamID, &lat, &lon, &radius, &typeId, &nameId)
		if err != nil {
			Log.Error(err)
			return s, nil
		}
		if Id.Valid {
			f, _ := strconv.ParseFloat(Id.String, 64)
			tmpTarget.ID = f
		}
		if nameId.Valid {
			tmpTarget.Desc = nameId.String
		}
		if lat.Valid {
			f, _ := strconv.ParseFloat(lat.String, 64)
			tmpTarget.Lat = f
		}
		if lon.Valid {
			f, _ := strconv.ParseFloat(lon.String, 64)
			tmpTarget.Lon = f
		}
		if radius.Valid {
			f, _ := strconv.ParseFloat(radius.String, 64)
			tmpTarget.Radius = f
		}
		tmpTarget.Share = true
		wp.Waypoints.Waypoints = append(wp.Waypoints.Waypoints, tmpTarget)
	}
	wps, _ := json.Marshal(wp)
	locs = append(locs, wps)

	s, _ = json.Marshal(locs)
	// Log.Debug(string(s))
	return s, nil
}

func OwnTracksTransition(gid string, transition json.RawMessage) (json.RawMessage, error) {
	var t Transition
	j := json.RawMessage("{ }")

	if err := json.Unmarshal(transition, &t); err != nil {
		Log.Notice(err)
		return j, err
	}

	// do something here
	Log.Debugf("%s transition %s: %s (%f)", gid, t.Event, t.Desc, t.ID)

	return j, nil
}

func ownTracksTidy(gid, otdata string) (json.RawMessage, error) {
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

func ownTracksExternalUpdate(gid, lat, lon string) error {
    var otdata string
    err := db.QueryRow("SELECT otdata FROM otdata WHERE gid = ?", gid).Scan(&otdata)

	var l Location
	if err := json.Unmarshal(json.RawMessage(otdata), &l); err != nil {
		Log.Notice(err)
		return  err
	}

	l.Lat, _ = strconv.ParseFloat(lat, 64)
	l.Lon, _ = strconv.ParseFloat(lon, 64)
	l.Topic = fmt.Sprintf("owntracks/%s/http", gid)

	t := time.Now()
	l.TimeStamp = float64(t.Unix())

	redo, err := json.Marshal(l)
	if err != nil {
		Log.Notice(err)
		return  err
	}

	_, err = db.Exec("UPDATE otdata SET otdata = ? WHERE gid = ?", redo, gid)
	if err != nil {
		Log.Notice(err)
		return  err
	}
	return nil
}

func OwnTracksSetWaypoint(gid string, wp json.RawMessage) (json.RawMessage, error) {
	Log.Debug(string(wp))
	var w Waypoint
	j := json.RawMessage("{ }")

	team, err := ownTracksDefaultTeam(gid) // cache this...
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

	return j, nil
}

func ownTracksWriteWaypoint(w Waypoint, team string) error {
	_, err := db.Exec("INSERT INTO target VALUES (?,?,POINT(?, ?),?,?,?,FROM_UNIXTIME(? + (86400 * 14)),NULL) ON DUPLICATE KEY UPDATE Id = ?, loc = POINT(?, ?), radius = ?, name = ?",
		w.ID, team, w.Lat, w.Lon, w.Radius, "target", w.Desc, w.ID,
		w.ID, w.Lat, w.Lon, w.Radius, w.Desc)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

func OwnTracksSetWaypointList(gid string, wp json.RawMessage) (json.RawMessage, error) {
	// Log.Debug(string(wp))
	var w WaypointsList
	j := json.RawMessage("{ }")

	team, err := ownTracksDefaultTeam(gid)
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

	return j, nil
}

func ownTracksDefaultTeam(gid string) (string, error) {
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
