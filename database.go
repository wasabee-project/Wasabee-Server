package PhDevBin

import (
	"database/sql"
	"time"
	// MySQL/MariaDB Database Driver
	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB
var locQuery *sql.Stmt
var isConnected bool

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

	// super-frequent query, have it always ready
	locQuery, err = db.Prepare("UPDATE locations SET loc = PointFromText(?), upTime = NOW() WHERE gid = ?")
	if err != nil {
		Log.Errorf("Couldn't initialize location statement: %s", err)
		return err
	}

	// Create tables
	var table string
	db.QueryRow("SHOW TABLES LIKE 'documents'").Scan(&table)
	if table == "" {
		Log.Noticef("Setting up `documents` table...")
		_, err := db.Exec(`CREATE TABLE documents (
            id varchar(64) PRIMARY KEY,
			uploader varchar(32) NULL DEFAULT NULL,
            content longblob NOT NULL,
            upload datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
            expiration datetime NULL DEFAULT NULL,
            views int UNSIGNED NOT NULL DEFAULT 0
        ) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin`)
		if err != nil {
			return err
		}
	}

	table = ""
	db.QueryRow("SHOW TABLES LIKE 'user'").Scan(&table)
	if table == "" {
		Log.Noticef("Setting up `user` table...")
		_, err := db.Exec(`CREATE TABLE user(
			gid varchar(32) PRIMARY KEY,
            iname varchar(64) NULL DEFAULT NULL,
            lockey varchar(64) NULL DEFAULT NULL
        ) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin`)
		if err != nil {
			return err
		}
	}

	table = ""
	db.QueryRow("SHOW TABLES LIKE 'locations'").Scan(&table)
	if table == "" {
		Log.Noticef("Setting up `locations` table...")
		_, err := db.Exec(`CREATE TABLE locations(
			gid varchar(32) PRIMARY KEY,
            upTime datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
            loc POINT
        ) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin`)
		if err != nil {
			return err
		}
	}

	table = ""
	db.QueryRow("SHOW TABLES LIKE 'tags'").Scan(&table)
	if table == "" {
		Log.Noticef("Setting up `tags` table...")
		_, err := db.Exec(`CREATE TABLE tags(
			tagID varchar(64) PRIMARY KEY,
			owner varchar(32) NOT NULL,
			name varchar(64) NULL DEFAULT NULL
        ) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin`)
		if err != nil {
			return err
		}
	}

	table = ""
	db.QueryRow("SHOW TABLES LIKE 'usertags'").Scan(&table)
	if table == "" {
		Log.Noticef("Setting up `usertags` table...")
		_, err := db.Exec(`CREATE TABLE usertags(
			tagID varchar(64) NOT NULL,
			gid varchar(32) NOT NULL,
			state ENUM('Off', 'On') NOT NULL DEFAULT 'Off', 
            color varchar(32) NOT NULL DEFAULT "FF5500",
			PRIMARY KEY (tagID, gid)
        ) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin`)
		if err != nil {
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
