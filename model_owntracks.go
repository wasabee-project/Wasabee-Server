package PhDevBin

import (
	"strconv"
	"encoding/json"
	"database/sql"
)

func OwnTracksUpdate(gid, jsonblob string, lat, lon float64) error {
    _, err := db.Exec("UPDATE otdata SET otdata = ? WHERE gid = ?", jsonblob, gid)
	if err != nil {
	    Log.Notice(err)
	}
	err = UserLocation(gid, strconv.FormatFloat(lat, 'f', -1, 64), strconv.FormatFloat(lon, 'f', -1, 64))
	return err
}

func OwnTracksTeams(gid string) (json.RawMessage, error) {
    var locs []json.RawMessage;
    var tmp sql.NullString

    r, err := db.Query("SELECT DISTINCT o.otdata from otdata=o, userteams=ut WHERE o.gid = ut.gid AND o.gid != ? AND ut.teamID IN (SELECT teamID FROM userteams WHERE gid = ?)", gid, gid)
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
             locs = append(locs, json.RawMessage(tmp.String))
        }
	}
    s, _ := json.Marshal(locs)
	// Log.Notice(string(s))
    return s, nil
}
