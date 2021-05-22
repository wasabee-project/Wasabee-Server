package wasabee

import (
	"database/sql"
	"fmt"

	// need a comment here to make lint happy
	_ "github.com/go-sql-driver/mysql"
)

// db is the private global used by all relevant functions to interact with the database
var db *sql.DB

// Connect tries to establish a connection to a MySQL/MariaDB database under the given URI and initializes the tables if they don"t exist yet.
func Connect(uri string) error {
	Log.Debugw("startup", "database uri", uri)
	result, err := sql.Open("mysql", uri)
	if err != nil {
		return err
	}
	db = result

	var version string
	if err := db.QueryRow("SELECT VERSION()").Scan(&version); err != nil {
		return err
	}
	Log.Infow("startup", "database", "connected", "version", version, "message", "connected to database")

	setupTables()
	upgradeTables()
	return nil
}

// Disconnect closes the database connection
// called only at server shutdown
func Disconnect() {
	Log.Infow("shutdown", "message", "cleanly disconnected from database")
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
		// agent must come first, team must come second, operation must come third, the rest can be in alphabetical order
		{"agent", `CREATE TABLE agent ( gid varchar(32) NOT NULL, name varchar(64) DEFAULT NULL, level tinyint(4) NOT NULL DEFAULT '1', OneTimeToken varchar(64) NOT NULL DEFAULT UUID(), VVerified tinyint(1) NOT NULL DEFAULT '0', Vblacklisted tinyint(1) NOT NULL DEFAULT '0', Vid varchar(40) DEFAULT NULL, RocksVerified tinyint(1) NOT NULL DEFAULT '0', RAID tinyint(1) NOT NULL DEFAULT '0', RISC tinyint(1) NOT NULL DEFAULT '0', PRIMARY KEY (gid), UNIQUE KEY name (name), UNIQUE KEY OneTimeToken (OneTimeToken), UNIQUE KEY Vid (Vid)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"team", `CREATE TABLE team ( teamID varchar(64) NOT NULL, owner varchar(32) NOT NULL, name varchar(64) DEFAULT NULL, rockskey varchar(32) DEFAULT NULL, rockscomm varchar(32) DEFAULT NULL, joinLinkToken varchar(64), telegram bigint signed, PRIMARY KEY (teamID), KEY fk_team_owner (owner), CONSTRAINT fk_team_owner FOREIGN KEY (owner) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"operation", `CREATE TABLE operation ( ID varchar(64) NOT NULL, name varchar(128) NOT NULL DEFAULT 'new op', gid varchar(32) NOT NULL, color varchar(16) NOT NULL DEFAULT 'green', teamID varchar(64) NOT NULL DEFAULT '', modified datetime NOT NULL DEFAULT CURRENT_TIMESTAMP, comment text, lasteditid varchar(40) NOT NULL DEFAULT 'unset', referencetime datetime NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (ID), KEY gid (gid), KEY teamID (teamID), CONSTRAINT fk_operation_agent FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`},

		{"agentextras", `CREATE TABLE agentextras ( gid varchar(32) NOT NULL, picurl text, UNIQUE KEY gid (gid), CONSTRAINT fk_extra_agent FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"agentteams", `CREATE TABLE agentteams ( teamID varchar(64) NOT NULL, gid varchar(32) NOT NULL, state enum('Off','On') NOT NULL DEFAULT 'Off', squad varchar(32) NOT NULL DEFAULT 'agents', shareWD enum('Off','On') NOT NULL DEFAULT 'Off', loadWD enum('Off','On') NOT NULL DEFAULT 'Off', PRIMARY KEY (teamID,gid), KEY GIDKEY (gid), CONSTRAINT fk_agent_teams FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE, CONSTRAINT fk_t_teams FOREIGN KEY (teamID) REFERENCES team (teamID) ON DELETE CASCADE ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`},
		{"firebase", `CREATE TABLE firebase ( gid varchar(32) NOT NULL, token varchar(4092) NOT NULL, KEY fk_gid (gid), CONSTRAINT fk_gid FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"link", `CREATE TABLE link ( ID varchar(64) NOT NULL, fromPortalID varchar(64) NOT NULL, toPortalID varchar(64) NOT NULL, opID varchar(64) NOT NULL, description text, gid varchar(32) DEFAULT NULL, throworder int(11) DEFAULT '0', completed tinyint(1) NOT NULL DEFAULT '0', color varchar(16) NOT NULL DEFAULT 'main', zone tinyint(4) NOT NULL DEFAULT 1, mu bigint unsigned NOT NULL DEFAULT 0, delta int NOT NULL default 0, PRIMARY KEY (ID,opID), KEY fk_operation_id_link (opID), KEY fk_link_gid (gid), CONSTRAINT fk_link_gid FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE SET NULL, CONSTRAINT fk_operation_id_link FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"locations", `CREATE TABLE locations ( gid varchar(32) NOT NULL, upTime datetime NOT NULL DEFAULT CURRENT_TIMESTAMP, loc point NOT NULL, PRIMARY KEY (gid)) DEFAULT CHARSET=utf8mb4;`},
		{"marker", `CREATE TABLE marker ( ID varchar(64) NOT NULL, opID varchar(64) NOT NULL, portalID varchar(64) NOT NULL, type varchar(128) NOT NULL, gid varchar(32) DEFAULT NULL, comment text, complete tinyint(1) NOT NULL DEFAULT '0', state enum('pending','assigned','acknowledged','completed') NOT NULL DEFAULT 'pending', completedBy varchar(32) DEFAULT NULL, oporder int NOT NULL DEFAULT 0, zone tinyint(4) NOT NULL DEFAULT 1,  delta int NOT NULL default 0, PRIMARY KEY (ID,opID), KEY fk_operation_marker (opID), KEY fk_marker_gid (gid), CONSTRAINT fk_marker_gid FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE SET NULL, CONSTRAINT fk_operation_marker FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"messagelog", `CREATE TABLE messagelog ( timestamp datetime NOT NULL DEFAULT CURRENT_TIMESTAMP, gid varchar(32) NOT NULL, message text NOT NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"opkeys", `CREATE TABLE opkeys ( opID varchar(64) NOT NULL, portalID varchar(64) NOT NULL, gid varchar(32) NOT NULL, onhand int(11) NOT NULL DEFAULT '0', capsule varchar(16) DEFAULT NULL, UNIQUE KEY key_unique (opID,portalID,gid), KEY fk_operation_id_keys (opID), CONSTRAINT fk_operation_id_keys FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"portal", `CREATE TABLE portal ( ID varchar(64) NOT NULL, opID varchar(64) NOT NULL, name varchar(128) NOT NULL, loc point NOT NULL, comment text, hardness varchar(64) DEFAULT NULL, PRIMARY KEY ID (ID,opID), KEY fk_operation_id (opID)) DEFAULT CHARSET=utf8mb4;`},
		{"telegram", `CREATE TABLE telegram ( telegramID bigint(20) NOT NULL, telegramName varchar(32) NOT NULL, gid varchar(32) NOT NULL, verified tinyint(1) NOT NULL DEFAULT '0', authtoken varchar(32) DEFAULT NULL, PRIMARY KEY (telegramID), UNIQUE KEY gid (gid), CONSTRAINT fk_agent_telegram FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"opteams", `CREATE TABLE opteams (teamID varchar(64) NOT NULL, opID varchar(64) NOT NULL, permission enum('read','write','assignedonly') NOT NULL DEFAULT 'read', zone tinyint(4) NOT NULL DEFAULT 0, KEY opID (opID), KEY teamID (teamID), CONSTRAINT fk_ops_teamID FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE, CONSTRAINT fk_teamIDs_op FOREIGN KEY (teamID) REFERENCES team (teamID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`},
		{"defensivekeys", `CREATE TABLE defensivekeys (gid varchar(32) NOT NULL, portalID varchar(64) NOT NULL, capID varchar(16) DEFAULT NULL, count int(3) NOT NULL DEFAULT '0', name varchar(128) DEFAULT NULL, loc point DEFAULT NULL, PRIMARY KEY (portalID, gid), KEY fk_dk_gid (gid), CONSTRAINT fk_dk_gid FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`},
		{"deletedops", `CREATE TABLE deletedops ( opID varchar(64) NOT NULL, deletedate datetime NOT NULL DEFAULT CURRENT_TIMESTAMP, gid varchar(32), PRIMARY KEY(opID)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`},
		{"zone", `CREATE TABLE zone ( ID tinyint(4) NOT NULL, opID varchar(64) NOT NULL, name varchar(64) NOT NULL DEFAULT 'zone', color varchar(10) NOT NULL DEFAULT 'green', PRIMARY KEY (ID,opID), KEY fk_operation_zone (opID), CONSTRAINT fk_operation_zone FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`},
		{"zonepoints", `CREATE TABLE zonepoints ( zoneID tinyint(4) NOT NULL, opID varchar(64) NOT NULL, position tinyint(4) NOT NULL, point point NOT NULL, PRIMARY KEY (zoneID,opID,position), KEY fk_operation_zonepoint (opID), CONSTRAINT fk_operation_zonepoint FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`},
	}

	var table string
	// use a tranaction to AVOID concurrency in this logic
	// it is possible for these to go in out-of-order and fk problems to show up under rare circumstances
	tx, err := db.Begin()
	if err != nil {
		Log.Error(err)
		panic(err)
	}
	_, err = tx.Exec("SET FOREIGN_KEY_CHECKS=0")
	if err != nil {
		Log.Error(err)
	}

	defer func() {
		err := tx.Rollback()
		if err != nil && err != sql.ErrTxDone {
			Log.Error(err)
		}
		// tx is complete, use db
		_, err = db.Exec("SET FOREIGN_KEY_CHECKS=1")
		if err != nil {
			Log.Error(err)
		}
	}()
	for _, v := range t {
		q := fmt.Sprintf("SHOW TABLES LIKE '%s'", v.tablename)
		err = tx.QueryRow(q).Scan(&table)
		if err != nil && err != sql.ErrNoRows {
			Log.Error(err)
			continue
		}
		if err == sql.ErrNoRows || table == "" {
			Log.Infof("Setting up '%s' table...", v.tablename)
			_, err = tx.Exec(v.creation)
			if err != nil {
				Log.Error(err)
			}
		}
	}
	_, err = tx.Exec("SET FOREIGN_KEY_CHECKS=1")
	if err != nil {
		Log.Error(err)
	}
	err = tx.Commit() // the defer'd rollback will not have anything to rollback...
	if err != nil {
		Log.Error(err)
	}
	// defer'd func runs here
}

func upgradeTables() {
	var upgrades = []struct {
		test    string // a query that will fail if an upgrade is needed
		upgrade string // the query to run to make the upgrade
	}{
		{"SELECT DATA_TYPE FROM information_schema.columns WHERE table_schema = Database() and table_name = 'operation' and column_name = 'lasteditid'", "ALTER TABLE operation ADD lasteditid varchar(40) NOT NULL DEFAULT 'unset'"},
		{"SELECT DATA_TYPE FROM information_schema.columns WHERE table_schema = Database() AND table_name = 'link' AND column_name = 'mu'", "ALTER TABLE link ADD mu bigint unsigned NOT NULL DEFAULT 0"},
		{"SELECT DATA_TYPE FROM information_schema.columns WHERE table_schema = Database() and table_name = 'link' and column_name = 'delta'", "ALTER TABLE link ADD delta int NOT NULL DEFAULT 0"},
		{"SELECT DATA_TYPE FROM information_schema.columns WHERE table_schema = Database() and table_name = 'marker' and column_name = 'delta'", "ALTER TABLE delta ADD delta int NOT NULL DEFAULT 0"},
		{"SELECT DATA_TYPE FROM information_schema.columns WHERE table_schema = Database() and table_name = 'operation' and column_name = 'referencetime'", "ALTER TABLE operation ADD referencetime datetime NOT NULL DEFAULT CURRENT_TIMESTAMP"},
		{"SELECT character_maximum_length FROM information_schema.columns WHERE table_schema = Database() AND table_name = 'opkeys' AND column_name = 'capsule' AND character_maximum_length = 16", "ALTER TABLE opkeys MODIFY capsule varchar(16) DEFAULT NULL"},
		{"SELECT character_maximum_length FROM information_schema.columns WHERE table_schema = Database() AND table_name = 'defensivekeys' AND column_name = 'capID' AND character_maximum_length = 16", "ALTER TABLE defensivekeys MODIFY capID varchar(16) DEFAULT NULL"},
		{"SELECT DATA_TYPE FROM information_schema.columns WHERE table_schema = Database() AND table_name = 'agentteams' AND column_name = 'squad'", "ALTER TABLE agentteams CHANGE color squad varchar(32) NOT NULL DEFAULT 'agents'"},
		{"SELECT DATA_TYPE FROM information_schema.columns WHERE table_schema = Database() AND table_name = 'agent' AND column_name = 'OneTimeToken'", "ALTER TABLE agent DROP INDEX lockey, DROP lockey, ADD OneTimeToken varchar(64) NOT NULL DEFAULT LEFT(UUID(),60), ADD UNIQUE (OneTimeToken)"},
		{"SELECT DATA_TYPE FROM information_schema.columns WHERE table_schema = Database() AND table_name = 'agent' AND column_name = 'name'", "ALTER TABLE agent CHANGE iname name varchar(64) NOT NULL"},
		{"SELECT DATA_TYPE FROM information_schema.columns WHERE table_schema = Database() AND table_name = 'agent' AND column_name = 'intelname'", "ALTER TABLE agent ADD intelname varchar(64) NOT NULL DEFAULT '', ADD intelfaction tinyint(1) NOT NULL DEFAULT '-1', ADD Vname varchar(64) NOT NULL DEFAULT '', ADD rocksname varchar(64) NOT NULL DEFAULT ''"},
		{"SELECT DATA_TYPE FROM information_schema.columns WHERE table_schema = Database() AND table_name = 'agentteams' AND column_name = 'shareWD'", "ALTER TABLE agentteams ADD shareWD enum('Off','On') NOT NULL DEFAULT 'Off', ADD loadWD enum('Off','On') NOT NULL DEFAULT 'Off'"},
		{"SELECT DATA_TYPE FROM information_schema.columns WHERE table_schema = Database() AND table_name = 'zone' AND column_name = 'color'", "ALTER TABLE zone ADD color varchar(10) NOT NULL DEFAULT 'green'"},
	}

	tx, err := db.Begin()
	if err != nil {
		Log.Error(err)
		panic(err)
	}
	_, err = tx.Exec("SET FOREIGN_KEY_CHECKS=0")
	if err != nil {
		Log.Error(err)
	}
	defer func() {
		err := tx.Rollback()
		if err != nil && err != sql.ErrTxDone {
			Log.Error(err)
		}
		_, err = db.Exec("SET FOREIGN_KEY_CHECKS=1")
		if err != nil {
			Log.Error(err)
		}
	}()

	// do upgrades
	var scratch string
	for _, q := range upgrades {
		err = tx.QueryRow(q.test).Scan(&scratch)
		if err == nil {
			continue
		}
		Log.Debugw("schema check failed", "test", q.test, "error", err.Error(), "doing upgrade", q.upgrade)
		_, err = tx.Exec(q.upgrade)
		if err != nil {
			Log.Error(err)
			panic(err)
		}
	}

	// all upgrades done...
	_, err = tx.Exec("SET FOREIGN_KEY_CHECKS=1")
	if err != nil {
		Log.Error(err)
	}
	err = tx.Commit()
	if err != nil {
		Log.Error(err)
	}
}

// MakeNullString is used for values that may & might be inserted/updated as NULL in the database
func MakeNullString(in interface{}) sql.NullString {
	var s string

	tmp, ok := in.(string)
	if ok {
		s = tmp
	} else {
		tmp, ok := in.(fmt.Stringer)
		if !ok {
			return sql.NullString{}
		}
		s = tmp.String()
	}
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{
		String: s,
		Valid:  true,
	}
}
