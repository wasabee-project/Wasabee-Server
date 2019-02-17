package PhDevBin

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/scrypt"

	"crypto/sha256"
)

const MaxFilesize = 1024 * 1024 // 1MB

// Document specifies the content and metadata of a piece of code that is hosted on PhDevBin.
type Document struct {
	// ID is set on Store()
	ID       string
	Uploader string
	Content  string
	// Upload is set on Store()
	Upload     time.Time
	Expiration time.Time
	Views      int
	UserID     string
}

// Store a document object in the database.
func Store(document *Document) error {
	// Generate a name that doesn't exist yet
	name, err := GenerateSafeName()
	if err != nil {
		return err
	}
	document.ID = name

	// Round the timestamps on the object. Won't affect the database, but we want consistency.
	document.Upload = time.Now().Round(time.Second)
	document.Expiration = document.Expiration.Round(time.Second)

	// Normalize new lines
	document.Content = strings.Trim(strings.Replace(strings.Replace(document.Content, "\r\n", "\n", -1), "\r", "\n", -1), "\n") + "\n"

	// Don't accept binary files
	if strings.Contains(document.Content, "\x00") {
		return errors.New("file contains 0x00 bytes")
	}

	escaped := ""
	escaped = EscapeHTML(document.Content)

	var expiration interface{}
	if (document.Expiration != time.Time{}) {
		expiration = document.Expiration.UTC().Format("2006-01-02 15:04:05")
	}

	// Server-Side Encryption
	key, err := scrypt.Key([]byte(document.ID), []byte(document.Upload.UTC().Format("2006-01-02 15:04:05")), 16384, 8, 1, 24)
	if err != nil {
		Log.Errorf("Invalid script parameters: %s", err)
	}
	data, err := encrypt([]byte(escaped), key)
	if err != nil {
		Log.Errorf("AES error: %s", err)
		return err
	}

	databaseID := sha256.Sum256([]byte(document.ID))

	// Write the document to the database
	// XXX add Uploader
	_, err = db.Exec(
		"INSERT INTO documents (id, uploader, content, upload, expiration, views) VALUES (?, ?, ?, ?, ?, ?)",
		hex.EncodeToString(databaseID[:]),
		document.UserID,
		string(data),
		document.Upload.UTC().Format("2006-01-02 15:04:05"),
		expiration,
		document.Views)
	if err != nil {
		return err
	}
	return nil
}

// Update a document object in the database.
func Update(document *Document) error {
	// Round the timestamps on the object. Won't affect the database, but we want consistency.
	document.Upload = time.Now().Round(time.Second)

	// Normalize new lines
	document.Content = strings.Trim(strings.Replace(strings.Replace(document.Content, "\r\n", "\n", -1), "\r", "\n", -1), "\n") + "\n"

	// Don't accept binary files
	if strings.Contains(document.Content, "\x00") {
		return errors.New("file contains 0x00 bytes")
	}

	escaped := ""
	escaped = EscapeHTML(document.Content)

	// Server-Side Encryption
	key, err := scrypt.Key([]byte(document.ID), []byte(document.Upload.UTC().Format("2006-01-02 15:04:05")), 16384, 8, 1, 24)
	if err != nil {
		Log.Errorf("Invalid script parameters: %s", err)
	}
	data, err := encrypt([]byte(escaped), key)
	if err != nil {
		Log.Errorf("AES error: %s", err)
		return err
	}

	databaseID := sha256.Sum256([]byte(document.ID))

	// Write the document to the database
	_, err = db.Exec(
		"UPDATE documents SET content=?, upload=? WHERE id=? AND uploader=?",
		string(data),
		document.Upload.UTC().Format("2006-01-02 15:04:05"),
		hex.EncodeToString(databaseID[:]),
		document.UserID)
	if err != nil {
		return err
	}
	return nil
}

// Request a document from the database by its ID.
func Request(id string) (Document, error) {
	doc := Document{ID: id}
	var views int
	var upload, expiration sql.NullString
	databaseID := sha256.Sum256([]byte(id))
	err := db.QueryRow("SELECT content, upload, expiration, views FROM documents WHERE id = ?", hex.EncodeToString(databaseID[:])).
		Scan(&doc.Content, &upload, &expiration, &views)
	if err != nil {
		if err.Error() != "sql: no rows in result set" {
			Log.Warningf("Error retrieving document: %s", err)
		}
		return Document{}, err
	}

	go db.Exec("UPDATE documents SET views = views + 1 WHERE id = ?", hex.EncodeToString(databaseID[:]))
	doc.Views = views

	doc.Upload, _ = time.Parse("2006-01-02 15:04:05", upload.String)

	key, err := scrypt.Key([]byte(id), []byte(doc.Upload.UTC().Format("2006-01-02 15:04:05")), 16384, 8, 1, 24)
	if err != nil {
		Log.Errorf("Invalid script parameters: %s", err)
		return Document{}, err
	}
	data, err := decrypt([]byte(doc.Content), key)
	if err != nil && !(err.Error() == "cipher: message authentication failed" && !strings.Contains(doc.Content, "\000")) {
		Log.Errorf("AES error: %s", err)
		return Document{}, err
	} else if err == nil {
		doc.Content = string(data)
	}

	if expiration.Valid {
		doc.Expiration, err = time.Parse("2006-01-02 15:04:05", expiration.String)
		if doc.Expiration.Before(time.Unix(0, 1)) {
			if doc.Views > 0 {
				// Volatile document
				_, err = db.Exec("DELETE FROM documents WHERE id = ?", hex.EncodeToString(databaseID[:]))
				if err != nil {
					Log.Errorf("Couldn't delete volatile document: %s", err)
				}
			}
		} else {
			if err != nil {
				return Document{}, err
			}
			if doc.Expiration.Before(time.Now()) {
				return Document{}, errors.New("the document has expired")
			}
		}
	}

	doc.Content = StripHTML(doc.Content)
	return doc, nil
}

func Delete(id string) error {
	databaseID := sha256.Sum256([]byte(id))
	_, err := db.Exec("DELETE FROM documents WHERE id = ?", hex.EncodeToString(databaseID[:]))
	if err != nil {
		if err.Error() != "sql: no rows in result set" {
			Log.Warningf("Error deleting document: %s", err)
		}
		return err
	}
	return nil
}

// user stuff
type UserData struct {
	GoogleName  string
	IngressName string
	LocationKey string
}

func InsertOrUpdateUser(id string, name string) error {
	lockey, err := GenerateSafeName()
	_, err = db.Exec("INSERT INTO user VALUES (?,?,NULL,?) ON DUPLICATE KEY UPDATE gname = ?", id, name, lockey, name)
	if err != nil {
		return err
	}
	_, err = db.Exec("INSERT INTO locations VALUES (?,NOW(),NULL) ON DUPLICATE KEY UPDATE upTime = NOW()", id)
	if err != nil {
		return err
	}
	return nil
}

func SetIngressName(id string, name string) error {
	_, err := db.Exec("UPDATE user SET iname = ? WHERE gid = ?", name, id)
	if err != nil {
		return err
	}
	return nil
}

func RemoveUserFromTag(id string, tag string) error {
	_, err := db.Exec("DELETE FROM usertags WHERE gid = ? AND tagID = ?", tag, id)
	if err != nil {
		return err
	}
	return nil
}

func SetUserTagState(id string, tag string, state string) error {
	if state != "On" {
		state = "Off"
	}
	_, err := db.Exec("UPDATE usertags SET state = ? WHERE gid = ? AND tagID = ?", state, tag, id)
	if err != nil {
		return err
	}
	return nil
}

func GetUserData(id string, ud *UserData) error {
	var gn, in, lc sql.NullString

	row := db.QueryRow("SELECT gname, iname, lockey FROM user WHERE gid = ?", id)
	err := row.Scan(&gn, &in, &lc)
	if err != nil {
		return err
	}

	// convert from sql.NullString to string in the struct
	if gn.Valid {
		ud.GoogleName = gn.String
	}
	if in.Valid {
		ud.IngressName = in.String
	}
	if lc.Valid {
		ud.LocationKey = lc.String
	}
	return nil
}

// tag stuff
type TagData struct {
	Agent string
	Name  string
	Color string
	Lat   string
	Lon   string
	Date  string
}

func UserInTag(id string, tag string) (bool, error) {
	var count string

	err := db.QueryRow("SELECT COUNT(*) FROM usertags WHERE tagID = ? AND gid = ?", tag, id).Scan(&count)
	if err != nil {
		return false, err
	}
	i, err := strconv.Atoi(count)
	if i < 1 {
		return false, nil
	}
	return true, nil
}

func FetchTag(tag string, tagList *[]TagData) error {
	var tagID, gid, iname, color, loc, uptime sql.NullString
	var tmp TagData

	rows, err := db.Query("SELECT t.tagID, u.gid, u.iname, x.color, l.loc, l.upTime FROM tags=t, usertags=x, user=u, locations=l WHERE t.tagID = ? AND t.tagID = x.tagID AND x.gid = u.gid AND x.gid = l.gid", tag)
	if err != nil {
		Log.Error(err)
		return err
	}

	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tagID, &gid, &iname, &color, &loc, &uptime)
		if err != nil {
			Log.Error(err)
			return err
		}
		if gid.Valid {
			tmp.Agent = gid.String
		} else {
			tmp.Agent = ""
		}
		if iname.Valid {
			tmp.Name = iname.String
		} else {
			tmp.Name = ""
		}
		if color.Valid {
			tmp.Color = color.String
		} else {
			tmp.Color = ""
		}
		if loc.Valid { // this will need love
			tmp.Lat = loc.String
		} else {
			tmp.Lat = ""
		}
		if loc.Valid { // this will need love
			tmp.Lon = loc.String
		} else {
			tmp.Lon = ""
		}
		if uptime.Valid { // this will need love
			tmp.Date = uptime.String
		} else {
			tmp.Date = ""
		}
		*tagList = append(*tagList, tmp)
	}
	err = rows.Err()
	if err != nil {
		Log.Error(err)
		return err
	}

	return nil
}
