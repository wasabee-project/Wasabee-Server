package wasabi

import (
	"database/sql"
	"fmt"

	// need a comment here to make lint happy
	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

// Connect tries to establish a connection to a MySQL/MariaDB database under the given URI and initializes the tables if they don"t exist yet.
func Connect(uri string) error {
	Log.Debugf("Connecting to database at %s", uri)
	result, err := sql.Open("mysql", uri)
	if err != nil {
		return err
	}
	db = result

	// Print database version
	var version string
	if err := db.QueryRow("SELECT VERSION()").Scan(&version); err != nil {
		return err
	}
	Log.Infof("Database version: %s", version)

	setupTables()
	return nil
}

// Disconnect closes the database connection
// called only at server shutdown
func Disconnect() {
	Log.Debug("Disconnecting from database")
	if err := db.Close(); err != nil {
		Log.Error(err)
	}
}

// setupTables checks for the existence of tables and creates them if needed
func setupTables() {
	var t = []struct {
		tablename string
		creation  string
	}{
		{"document", `CREATE TABLE document (id varchar(64) PRIMARY KEY, content longblob NOT NULL, upload datetime NOT NULL DEFAULT CURRENT_TIMESTAMP, expiration datetime NULL DEFAULT NULL, views int UNSIGNED NOT NULL DEFAULT 0) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin`},
		{"agent", `CREATE TABLE agent (gid varchar(32) NOT NULL, iname varchar(64) DEFAULT NULL, level tinyint(4) NOT NULL DEFAULT "1", lockey varchar(64) DEFAULT NULL, OTpassword varchar(64) DEFAULT NULL, VVerified tinyint(1) NOT NULL DEFAULT "0", Vblacklisted tinyint(1) NOT NULL DEFAULT "0", Vid varchar(64) NOT NULL DEFAULT "", RocksVerified tinyint(1) NOT NULL DEFAULT "0", RAID tinyint(1) NOT NULL DEFAULT "0", RISC tinyint(1) NOT NULL DEFAULT "0", PRIMARY KEY (gid), UNIQUE KEY iname (iname), UNIQUE KEY lockey (lockey)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`},
		{"operation", `CREATE TABLE operation (ID varchar(64) NOT NULL, name varchar(128) NOT NULL DEFAULT "new op", gid varchar(32) NOT NULL, color varchar(16) NOT NULL DEFAULT "00FF00", teamID varchar(64) NOT NULL DEFAULT "", modified DATETIME NOT NULL DEFAULT NOW(), comment TEXT, PRIMARY KEY (ID), KEY gid (gid), CONSTRAINT fk_operation_agent FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`},
		{"team", `CREATE TABLE team (teamID varchar(64) NOT NULL, owner varchar(32) NOT NULL, name varchar(64) DEFAULT NULL, rockskey varchar(32) DEFAULT NULL, rockscomm varchar(32) DEFAULT NULL, PRIMARY KEY (teamID), KEY fk_team_owner (owner), CONSTRAINT fk_team_owner FOREIGN KEY (owner) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`},
		{"agentteams", `CREATE TABLE agentteams (teamID varchar(64) NOT NULL, gid varchar(32) NOT NULL, state enum("Off","On","Primary") NOT NULL DEFAULT "Off", color varchar(32) NOT NULL DEFAULT "FF5500", PRIMARY KEY (teamID,gid), KEY GIDKEY (gid), CONSTRAINT fk_agent_teams FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE, CONSTRAINT fk_t_teams FOREIGN KEY (teamID) REFERENCES team (teamID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`},
		{"anchor", `CREATE TABLE anchor (opID varchar(64) DEFAULT NULL, portalID varchar(64) DEFAULT NULL, UNIQUE KEY anchor (opID,portalID), CONSTRAINT fk_operation_id_anchor FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`},
		{"link", `CREATE TABLE link (ID varchar(64) NOT NULL, fromPortalID varchar(64) NOT NULL, toPortalID varchar(64) NOT NULL, opID varchar(64) NOT NULL, description text, gid varchar(32) DEFAULT NULL, throworder int(11) DEFAULT '0', KEY fk_operation_id_link (opID), CONSTRAINT fk_operation_id_link FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`},
		{"locations", `CREATE TABLE locations (gid varchar(32) NOT NULL, upTime datetime NOT NULL DEFAULT CURRENT_TIMESTAMP, loc point NOT NULL, PRIMARY KEY (gid), SPATIAL KEY sp (loc)) ENGINE=Aria DEFAULT CHARSET=utf8mb4 PAGE_CHECKSUM=1`},
		{"marker", `CREATE TABLE marker (ID varchar(64) NOT NULL, opID varchar(64) NOT NULL, portalID varchar(64) NOT NULL, type varchar(128) NOT NULL, gid varchar(32) DEFAULT NULL, comment text, complete tinyint(1) NOT NULL DEFAULT 0, KEY fk_operation_marker (opID), CONSTRAINT fk_operation_marker FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`},
		{"otdata", `CREATE TABLE otdata (gid varchar(32) NOT NULL, otdata text NOT NULL, PRIMARY KEY (gid), CONSTRAINT fk_otdata_gid FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`},
		{"portal", `CREATE TABLE portal (ID varchar(64) NOT NULL, opID varchar(64) NOT NULL, name varchar(128) NOT NULL, loc point NOT NULL, comment text, hardness varchar(64), UNIQUE KEY ID (ID,opID), KEY fk_operation_id (opID), SPATIAL KEY sp_portal (loc)) ENGINE=Aria DEFAULT CHARSET=utf8mb4 PAGE_CHECKSUM=1`},
		{"telegram", `CREATE TABLE telegram (telegramID bigint(20) NOT NULL, telegramName varchar(32) NOT NULL, gid varchar(32) NOT NULL, verified tinyint(1) NOT NULL DEFAULT "0", authtoken varchar(32) DEFAULT NULL, PRIMARY KEY (telegramID), UNIQUE KEY gid (gid), CONSTRAINT fk_agent_telegram FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`},
		{"waypoints", `CREATE TABLE waypoints (Id bigint(20) NOT NULL, teamID varchar(64) CHARACTER SET latin1 NOT NULL, loc point NOT NULL, radius int(10) unsigned NOT NULL DEFAULT "60", type varchar(32) CHARACTER SET latin1 NOT NULL DEFAULT "target", name varchar(128) CHARACTER SET latin1 DEFAULT NULL, expiration datetime NOT NULL, PRIMARY KEY (Id), KEY teamID (teamID), SPATIAL KEY sp (loc)) ENGINE=Aria DEFAULT CHARSET=utf8mb4 PAGE_CHECKSUM=1`},
		{"messagelog", `CREATE TABLE messagelog (timestamp datetime NOT NULL DEFAULT CURRENT_TIMESTAMP, gid varchar(32) NOT NULL, message text NOT NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`},
		{"opkeys", `CREATE TABLE opkeys ( opID varchar(64) NOT NULL, portalID varchar(64) NOT NULL, gid varchar(32) NOT NULL, onhand int(11) NOT NULL DEFAULT '0', UNIQUE KEY key_unique (opID,portalID,gid), KEY fk_operation_id_keys (opID), CONSTRAINT fk_operation_id_keys FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`},
		{"agentextras", `CREATE TABLE agentextras (gid varchar(32) NOT NULL, picurl TEXT, UNIQUE KEY gid (gid), CONSTRAINT fk_extra_agent FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`},
	}

	var table string
	// use a tranaction to AVOID concurrency in this logic
	// it is possible for these to go in out-of-order and fk problems to show up under rare circumstances
	tx, err := db.Begin()
	if err != nil {
		Log.Error(err)
		panic(err)
	}
	defer tx.Rollback()
	for _, v := range t {
		q := fmt.Sprintf("SHOW TABLES LIKE '%s'", v.tablename)
		err = tx.QueryRow(q).Scan(&table)
		if err != nil && err != sql.ErrNoRows {
			Log.Error(err)
			continue
		}
		if err == sql.ErrNoRows || table == "" {
			Log.Noticef("Setting up '%s' table...", v.tablename)
			_, err = tx.Exec(v.creation)
			if err != nil {
				Log.Error(err)
			}
		}
	}
	_ = tx.Commit() // the defer'd rollback will not have anything to rollback...
}
