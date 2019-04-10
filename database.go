package WASABI

import (
	"database/sql"
	"time"
	// need a comment here to make lint happy
	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

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
	Log.Infof("Database version: %s", version)

	err = setupTables()
	if err != nil {
		Log.Error(err)
	}
	return nil
}

// setupTables checks for the existence of tables and creates them if needed
// XXX THIS IS CURRENTLY OUT OF SYNC WITH REALITY
func setupTables() error {
	// Create tables
	var table string
	db.QueryRow("SHOW TABLES LIKE 'document'").Scan(&table)
	if table == "" {
		Log.Noticef("Setting up `document` table...")
		_, err := db.Exec(`CREATE TABLE document (
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
	return nil
}
