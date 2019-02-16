package PhDevBin

import (
	"database/sql"
	"time"
	// MySQL/MariaDB Database Driver
	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB
var isConnected bool

type UserData struct {
	GoogleName  string
	IngressName string
	LocationKey string
}

// Connect tries to establish a connection to a MySQL/MariaDB database under the given URI and initializes the tables if they don't exist yet.
func Connect(uri string) error {
	Log.Noticef("Connecting to database at %s", uri)
	result, err := try(func() (interface{}, error) {
		return sql.Open("mysql", uri)
	}, 10, time.Second) // Wait up to 10 seconds for the database
	if err != nil {
		return err
	}
	db = result.(*sql.DB)

	// Print database version
	var version string
	_, err = try(func() (interface{}, error) {
		return nil, db.QueryRow("SELECT VERSION()").Scan(&version)
	}, 10, time.Second) // Wait up to 10 seconds for the database
	if err != nil {
		return err
	}
	Log.Noticef("Database version: %s", version)

	// Create tables
	var table string
	db.QueryRow("SHOW TABLES LIKE 'documents'").Scan(&table)
	if table == "" {
		Log.Noticef("Setting up `documents` table...")
		err = db.QueryRow(`CREATE TABLE documents (
            id varchar(64) PRIMARY KEY,
			uploader varchar(32) NULL DEFAULT NULL,
            content longblob NOT NULL,
            upload datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
            expiration datetime NULL DEFAULT NULL,
            views int UNSIGNED NOT NULL DEFAULT 0
        ) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin`).Scan()
		if err != nil && err.Error() != "sql: no rows in result set" {
			return err
		}
	}

	table = ""
	db.QueryRow("SHOW TABLES LIKE 'user'").Scan(&table)
	if table == "" {
		Log.Noticef("Setting up `user` table...")
		err = db.QueryRow(`CREATE TABLE user(
			gid varchar(32) PRIMARY KEY,
            gname varchar(128) NOT NULL,
            iname varchar(64) NULL DEFAULT NULL,
            lockey varchar(64) NULL DEFAULT NULL
        ) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin`).Scan()
		if err != nil && err.Error() != "sql: no rows in result set" {
			return err
		}
	}

	table = ""
	db.QueryRow("SHOW TABLES LIKE 'locations'").Scan(&table)
	if table == "" {
		Log.Noticef("Setting up `locations` table...")
		err = db.QueryRow(`CREATE TABLE locations(
			gid varchar(32) PRIMARY KEY,
            upTime datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
            loc POINT
        ) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin`).Scan()
		if err != nil && err.Error() != "sql: no rows in result set" {
			return err
		}
	}

	table = ""
	db.QueryRow("SHOW TABLES LIKE 'tags'").Scan(&table)
	if table == "" {
		Log.Noticef("Setting up `tags` table...")
		err = db.QueryRow(`CREATE TABLE tags(
			tagID varchar(64) PRIMARY KEY,
			owner varchar(32) NOT NULL,
			name varchar(64) NULL DEFAULT NULL
        ) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin`).Scan()
		if err != nil && err.Error() != "sql: no rows in result set" {
			return err
		}
	}

	table = ""
	db.QueryRow("SHOW TABLES LIKE 'usertags'").Scan(&table)
	if table == "" {
		Log.Noticef("Setting up `usertags` table...")
		err = db.QueryRow(`CREATE TABLE usertags(
			tagID varchar(64) PRIMARY KEY,
			gid varchar(32) NOT NULL,
			state ENUM('Off', 'On') NOT NULL DEFAULT 'Off', 
            color varchar(32) NOT NULL DEFAULT "FF5500"
        ) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin`).Scan()
		if err != nil && err.Error() != "sql: no rows in result set" {
			return err
		}
	}

	safeName, errSafeName = db.Prepare("SELECT COUNT(id) FROM documents WHERE id = ?")

	isConnected = true
	go cleanup()
	return nil
}

// IsConnected returns true if the database has already been initialized.
func IsConnected() bool {
	return isConnected
}

func cleanup() {
	stmt, err := db.Prepare("DELETE FROM documents WHERE expiration < CURRENT_TIMESTAMP AND expiration > FROM_UNIXTIME(0)")
	if err != nil {
		Log.Errorf("Couldn't initialize cleanup statement: %s", err)
		return
	}

	for {
		result, err := stmt.Exec()
		if err != nil {
			Log.Errorf("Couldn't execute cleanup statement: %s", err)
		} else {
			n, err := result.RowsAffected()
			if err == nil && n > 0 {
				Log.Debugf("Cleaned up %d documents.", n)
			}
		}

		time.Sleep(10 * time.Minute)
	}
}

func InsertOrUpdateUser(id string, name string) error {
	lockey, err := GenerateSafeName()
	_, err = db.Exec("INSERT INTO user VALUES (?,?,NULL,?) ON DUPLICATE KEY UPDATE gname = ?", id, name, lockey, name)
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
