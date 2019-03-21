package PhDevBin

import (
	"database/sql"
	"time"
	// MySQL/MariaDB Database Driver
	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB
var lockeyToGid *sql.Stmt
var safeName *sql.Stmt

// Connect tries to establish a connection to a MySQL/MariaDB database under the given URI and initializes the tables if they don't exist yet.
func Connect(uri string) error {
	Log.Debugf("Connecting to database at %s", uri)
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
		_, err := db.Exec(`CREATE TABLE documents (
			id varchar(64) PRIMARY KEY,
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
			lockey varchar(64) NULL DEFAULT NULL,
			OTpassword varchar(64) NULL DEFAULT NULL,
			VVerified BOOLEAN NOT NULL DEFAULT 0,
			VBlacklisted BOOLEAN NOT NULL DEFAULT 0
		) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin`)
		if err != nil {
			return err
		}
	}

	table = ""
	db.QueryRow("SHOW TABLES LIKE 'locations'").Scan(&table)
	if table == "" {
		Log.Noticef("Setting up `locations` table...")
		_, err := db.Exec(`CREATE TABLE locations (
			gid varchar(32) COLLATE utf8mb4_bin NOT NULL,
			upTime datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
			loc point NOT NULL,
			PRIMARY KEY (gid),
			SPATIAL KEY sp (loc)
	 	) ENGINE=Aria DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin PAGE_CHECKSUM=1`)
		if err != nil {
			return err
		}
	}

	table = ""
	db.QueryRow("SHOW TABLES LIKE 'teams'").Scan(&table)
	if table == "" {
		Log.Noticef("Setting up `teams` table...")
		_, err := db.Exec(`CREATE TABLE teams(
			teamID varchar(64) PRIMARY KEY,
			owner varchar(32) NOT NULL,
			name varchar(64) NULL DEFAULT NULL
		) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin`)
		if err != nil {
			return err
		}
	}

	table = ""
	db.QueryRow("SHOW TABLES LIKE 'userteams'").Scan(&table)
	if table == "" {
		Log.Noticef("Setting up `userteams` table...")
		_, err := db.Exec(`CREATE TABLE userteams(
			teamID varchar(64) NOT NULL,
			gid varchar(32) NOT NULL,
			state ENUM('Off', 'On', 'Primary') NOT NULL DEFAULT 'Off', 
			color varchar(32) NOT NULL DEFAULT "FF5500",
			PRIMARY KEY (teamID, gid)
		) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin`)
		if err != nil {
			return err
		}
	}

	table = ""
	db.QueryRow("SHOW TABLES LIKE 'otdata'").Scan(&table)
	if table == "" {
		Log.Noticef("Setting up `otdata` table...")
		_, err := db.Exec(`CREATE TABLE otdata(
			gid varchar(32) NOT NULL PRIMARY KEY,
			otdata TEXT
		) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin`)
		if err != nil {
			return err
		}
	}

	table = ""
	db.QueryRow("SHOW TABLES LIKE 'target'").Scan(&table)
	if table == "" {
		Log.Noticef("Setting up `target` table...")
		_, err := db.Exec(`CREATE TABLE target (
			Id bigint(20) NOT NULL,
			TeamID varchar(64) NOT NULL,
			loc point NOT NULL,
			radius int(10) unsigned NOT NULL DEFAULT '60',
			type varchar(32) NOT NULL DEFAULT 'target',
			name varchar(128) DEFAULT NULL,
			expiration datetime NOT NULL,
			linkdst varchar(128) DEFAULT NULL,
			PRIMARY KEY (Id),
			KEY teamID (teamID),
			SPATIAL KEY sp (loc)
		) ENGINE=Aria DEFAULT CHARSET=latin1 PAGE_CHECKSUM=1`)
		if err != nil {
			return err
		}
	}

	table = ""
	db.QueryRow("SHOW TABLES LIKE 'telegram'").Scan(&table)
	if table == "" {
		Log.Noticef("Setting up `telegram` table...")
		_, err := db.Exec(`CREATE TABLE telegram(
			telegramID BIGINT NOT NULL PRIMARY KEY,
			telegramName varchar(32) NOT NULL,
			gid varchar(32) NOT NULL,
			verified BOOLEAN NOT NULL DEFAULT 0, 
			authtoken varchar(32),
			KEY (gid)
		) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin`)
		if err != nil {
			return err
		}
	}

	safeName, err = db.Prepare("SELECT COUNT(id) FROM documents WHERE id = ?")
	if err != nil {
		Log.Errorf("Couldn't initialize safeName: %s", err)
		return err
	}

	lockeyToGid, err = db.Prepare("SELECT gid FROM user WHERE lockey = ?")
	if err != nil {
		Log.Errorf("Couldn't initialize lockeyToGid: %s", err)
		return err
	}

	go cleanup()
	return nil
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
