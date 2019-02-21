package PhDevBin

import (
	"database/sql"
)

// user stuff
type UserData struct {
	IngressName string
	LocationKey string
	Tags        []struct {
		Id    string
		Name  string
		State string
	}
	OwnedTags []struct {
		Name string
		Tag  string
	}
	OwnedDraws []struct {
		Hash       string
		AuthTag    string
		UploadTime string
		Expiration string
		Views      string
	}
}

func InsertOrUpdateUser(id string, name string) error {
	var tmpName = "Agent_" + id[:5]
	lockey, err := GenerateSafeName()
	_, err = db.Exec("INSERT INTO user VALUES (?,?,?) ON DUPLICATE KEY UPDATE gid = ?", id, tmpName, lockey, id)
	if err != nil {
		Log.Notice(err)
		return err
	}
	_, err = db.Exec("INSERT INTO locations VALUES (?,NOW(),POINT(0,0)) ON DUPLICATE KEY UPDATE upTime = NOW()", id)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

func SetIngressName(id string, name string) error {
	_, err := db.Exec("UPDATE user SET iname = ? WHERE gid = ?", name, id)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

func RemoveUserFromTag(id string, tag string) error {
	_, err := db.Exec("DELETE FROM usertags WHERE gid = ? AND tagID = ?", tag, id)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

func SetUserTagState(id string, tag string, state string) error {
	if state != "On" {
		state = "Off"
	}
	_, err := db.Exec("UPDATE usertags SET state = ? WHERE gid = ? AND tagID = ?", state, id, tag)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

func GetUserData(id string, ud *UserData) error {
	var in, lc sql.NullString

	row := db.QueryRow("SELECT iname, lockey FROM user WHERE gid = ?", id)
	err := row.Scan(&in, &lc)
	if err != nil {
		Log.Notice(err)
		return err
	}

	// convert from sql.NullString to string in the struct
	if in.Valid {
		ud.IngressName = in.String
	}
	if lc.Valid {
		ud.LocationKey = lc.String
	}

	var tagID, name, state sql.NullString
	var tmp struct {
		Id    string
		Name  string
		State string
	}

	rows, err := db.Query("SELECT t.tagID, t.name, x.state "+
		"FROM tags=t, usertags=x "+
		"WHERE x.gid = ? AND x.tagID = t.tagID", id)
	if err != nil {
		Log.Error(err)
		return err
	}

	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tagID, &name, &state)
		if err != nil {
			Log.Error(err)
			return err
		}
		if tagID.Valid {
			tmp.Id = tagID.String
		} else {
			tmp.Id = ""
		}
		if name.Valid {
			tmp.Name = name.String
		} else {
			tmp.Name = ""
		}
		if state.Valid {
			tmp.State = state.String
		} else {
			tmp.State = ""
		}
		ud.Tags = append(ud.Tags, tmp)
	}

	var tmpO struct {
		Name string
		Tag  string
	}
	rowsO, err := db.Query("SELECT tagID, name FROM tags WHERE owner = ?", id)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rowsO.Close()
	for rowsO.Next() {
		err := rowsO.Scan(&tagID, &name)
		if err != nil {
			Log.Error(err)
			return err
		}
		if tagID.Valid {
			tmpO.Tag = tagID.String
		} else {
			tmpO.Tag = ""
		}
		if name.Valid {
			tmpO.Name = name.String
		} else {
			tmpO.Name = ""
		}
		ud.OwnedTags = append(ud.OwnedTags, tmpO)
	}

	var tmpDoc struct {
		Hash       string
		AuthTag    string
		UploadTime string
		Expiration string
		Views      string
	}
	rows1, err := db.Query("SELECT id, authtag, upload, expiration, views FROM documents WHERE uploader = ?", id)
	if err != nil {
		Log.Error(err)
		return err
	}
	var docID, authtag, upload, expiration, views sql.NullString
	defer rows1.Close()
	for rows1.Next() {
		err := rows1.Scan(&docID, &authtag, &upload, &expiration, &views)
		if err != nil {
			Log.Error(err)
			return err
		}
		if docID.Valid {
			tmpDoc.Hash = docID.String
		} else {
			tmpDoc.Hash = ""
		}
		if authtag.Valid {
			tmpDoc.AuthTag = authtag.String
		} else {
			tmpDoc.AuthTag = ""
		}
		if upload.Valid {
			tmpDoc.UploadTime = upload.String
		} else {
			tmpDoc.UploadTime = ""
		}
		if expiration.Valid {
			tmpDoc.Expiration = expiration.String
		} else {
			tmpDoc.Expiration = ""
		}
		if views.Valid {
			tmpDoc.Views = views.String
		} else {
			tmpDoc.Views = ""
		}
		ud.OwnedDraws = append(ud.OwnedDraws, tmpDoc)
	}

	return nil
}
