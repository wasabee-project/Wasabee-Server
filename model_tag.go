package PhDevBin

import (
	"database/sql"
	"strconv"
)

// tag stuff
type TagData struct {
	User []struct {
		Id     string
		Name   string
		Color  string
		State  string // enum On Off
		LocKey string
		Lat    string
		Lon    string
		Date   string
	}
	Target []struct {
		Name       string
		PortalID   string
		Lat        string
		Lon        string
		Range      int    // in meters
		Kind       string // enum ?
		AssignedTo string
	}
}

func UserInTag(id string, tag string, allowOff bool) (bool, error) {
	var count string

	var err error
	if allowOff {
		err = db.QueryRow("SELECT COUNT(*) FROM usertags WHERE tagID = ? AND gid = ?", tag, id).Scan(&count)
	} else {
		err = db.QueryRow("SELECT COUNT(*) FROM usertags WHERE tagID = ? AND gid = ? AND state = 'On'", tag, id).Scan(&count)
	}
	if err != nil {
		return false, err
	}
	i, err := strconv.Atoi(count)
	if i < 1 {
		return false, nil
	}
	return true, nil
}

func FetchTag(tag string, tagList *TagData, fetchAll bool) error {
	var tagID, iname, color, state, lockey, lat, lon, uptime sql.NullString
	var tmp struct {
		Id     string
		Name   string
		Color  string
		State  string
		LocKey string
		Lat    string
		Lon    string
		Date   string
	}

	var err error
	var rows *sql.Rows
	if fetchAll != true {
		rows, err = db.Query("SELECT t.tagID, u.iname, u.lockey, x.color, x.state, X(l.loc), Y(l.loc), l.upTime "+
			"FROM tags=t, usertags=x, user=u, locations=l "+
			"WHERE t.tagID = ? AND t.tagID = x.tagID AND x.gid = u.gid AND x.gid = l.gid AND x.state = 'On'", tag)
	} else {
		rows, err = db.Query("SELECT t.tagID, u.iname, u.lockey, x.color, x.state, X(l.loc), Y(l.loc), l.upTime "+
			"FROM tags=t, usertags=x, user=u, locations=l "+
			"WHERE t.tagID = ? AND t.tagID = x.tagID AND x.gid = u.gid AND x.gid = l.gid", tag)
	}
	if err != nil {
		Log.Error(err)
		return err
	}

	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tagID, &iname, &lockey, &color, &state, &lat, &lon, &uptime)
		if err != nil {
			Log.Error(err)
			return err
		}
		if tagID.Valid {
			tmp.Id = tagID.String
		} else {
			tmp.Id = ""
		}
		if iname.Valid {
			tmp.Name = iname.String
		} else {
			tmp.Name = ""
		}
		if lockey.Valid {
			tmp.LocKey = lockey.String
		} else {
			tmp.LocKey = ""
		}
		if color.Valid {
			tmp.Color = color.String
		} else {
			tmp.Color = ""
		}
		if state.Valid {
			tmp.State = state.String
		} else {
			tmp.State = "Off"
		}
		if lat.Valid { // this will need love
			tmp.Lat = lat.String
		} else {
			tmp.Lat = "0"
		}
		if lon.Valid { // this will need love
			tmp.Lon = lon.String
		} else {
			tmp.Lon = "0"
		}
		if uptime.Valid { // this will need love
			tmp.Date = uptime.String
		} else {
			tmp.Date = ""
		}
		tagList.User = append(tagList.User, tmp)
	}
	err = rows.Err()
	if err != nil {
		Log.Error(err)
		return err
	}

	return nil
}

func UserOwnsTag(id string, tag string) (bool, error) {
	var owner string

	err := db.QueryRow("SELECT owner FROM tags WHERE tagID = ?", tag).Scan(&owner)
	// returning err w/o checking is lazy, but same result
	if id == owner {
		return true, err
	}
	return false, err
}

func NewTag(name string, id string) (string, error) {
	tag, err := GenerateSafeName()
	if err != nil {
		Log.Notice(err)
		return "", err
	}
	_, err = db.Exec("INSERT INTO tags VALUES (?,?,?)", tag, id, name)
	if err != nil {
		Log.Notice(err)
	}
	_, err = db.Exec("INSERT INTO usertags VALUES (?,?,'On','FF0000')", tag, id)
	if err != nil {
		Log.Notice(err)
	}
	return name, err
}

func DeleteTag(tagID string) error {
	_, err := db.Exec("DELETE FROM tags WHERE tagID = ?", tagID)
	if err != nil {
		Log.Notice(err)
	}
	_, err = db.Exec("DELETE FROM usertags WHERE tagID = ?", tagID)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

func AddUserToTag(tagID string, id string) error {
	var gid sql.NullString
	err := db.QueryRow("SELECT gid FROM user WHERE lockey = ?", id).Scan(&gid)
	if err != nil {
		Log.Notice(id)
		Log.Notice(err)
		return err
	}

	_, err = db.Exec("INSERT INTO usertags values (?, ?, 'Off', '')", tagID, gid)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

func DelUserFromTag(tagID string, id string) error {
	var gid sql.NullString
	err := db.QueryRow("SELECT gid FROM user WHERE lockey = ?", id).Scan(&gid)
	if err != nil {
		Log.Notice(id)
		Log.Notice(err)
		return err
	}

	_, err = db.Exec("DELETE FROM usertags WHERE tagID = ? AND gid = ?", tagID, gid)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

func UserLocation(id string, lat string, lon string) error {
	var point string
	// sanity checing on bounds?
	point = "POINT(" + lat + " " + lon + ")"
	_, err := locQuery.Exec(point, id)
	if err != nil {
		Log.Notice(err)
	}
	return err
}
