package PhDevBin

import (
	"database/sql"
	"encoding/json"
	"strconv"
)

type WaypointCommand struct {
	Type      string `json:"_type"`
	Action    string `json:"action"`
	Waypoints struct {
		Type      string     `json:"_type"`
		Waypoints []Waypoint `json:"waypoints"`
	} `json:"waypoints"`
}

type Waypoint struct {
	Type   string  `json:"_type"`
	Desc   string  `json:"desc"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	Radius float64 `json:"rad"`
	ID     float64 `json:"tst"`
	UUID   string  `json:"uuid"`
	Major  string  `json:"major"`
	Minor  string  `json:"minor"`
}

// every query in here should be prepared since these are called VERY frequently
// data is vague on prepared statement performance inprovement -- do real testing
func OwnTracksUpdate(gid, jsonblob string, lat, lon float64) error {
	_, err := db.Exec("UPDATE otdata SET otdata = ? WHERE gid = ?", jsonblob, gid)
	if err != nil {
		Log.Notice(err)
	}
	err = UserLocation(gid, strconv.FormatFloat(lat, 'f', -1, 64), strconv.FormatFloat(lon, 'f', -1, 64))
	return err
}

func OwnTracksTeams(gid string) (json.RawMessage, error) {
	var locs []json.RawMessage
	var tmp sql.NullString

	r, err := db.Query("SELECT DISTINCT o.otdata FROM otdata=o, userteams=ut, locations=l WHERE o.gid = ut.gid AND o.gid != ? AND ut.teamID IN (SELECT teamID FROM userteams WHERE gid = ? AND state = 'On') AND ut.state = 'On' AND l.upTime > SUBTIME(NOW(), '12:00:00')", gid, gid)
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
			clean, _ := ownTracksTidy(gid, tmp.String)
			locs = append(locs, clean)
		}
	}
	s, _ := json.Marshal(locs)

	var wp WaypointCommand
	wp.Type = "cmd"
	wp.Action = "setWaypoints"
	var Id, teamID, lat, lon, radius, typeId, nameId sql.NullString
	var tmpTarget Waypoint
	tmpTarget.Type = "waypoint"

	wr, err := db.Query("SELECT Id, t.teamID, X(loc) as lat, Y(loc) as lon, radius, type, name FROM target=t, userteams=ut WHERE ut.teamID = t.teamID AND ut.teamID IN (SELECT teamID FROM userteams WHERE ut.gid = ? AND ut.state = 'On')", gid)
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
		ttt, _ := json.Marshal(tmpTarget)
		Log.Debug(string(ttt))
		wp.Waypoints.Waypoints = append(wp.Waypoints.Waypoints, tmpTarget)
	}
	wps, _ := json.Marshal(wp)
	locs = append(locs, wps)

	s, _ = json.Marshal(locs)
	Log.Notice(string(s))
	return s, nil
}

func ownTracksTidy(gid, otdata string) (json.RawMessage, error) {
	// if we need -- parse and clean the data, for now just returning it is fine
	return json.RawMessage(otdata), nil
}
