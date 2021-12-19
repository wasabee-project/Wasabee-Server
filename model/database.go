package model

import (
	"database/sql"
	"fmt"

	// need a comment here to make lint happy
	_ "github.com/go-sql-driver/mysql"
	"github.com/wasabee-project/Wasabee-Server/log"
)

// db is the private global used by all relevant functions to interact with the database
var db *sql.DB

// Connect tries to establish a connection to a MySQL/MariaDB database under the given URI and initializes the tables if they don"t exist yet.
func Connect(uri string) error {
	log.Debugw("startup", "database uri", uri)
	result, err := sql.Open("mysql", uri)
	if err != nil {
		return err
	}
	db = result

	var version string
	if err := db.QueryRow("SELECT VERSION()").Scan(&version); err != nil {
		return err
	}
	log.Infow("startup", "database", "connected", "version", version, "message", "connected to database")

	setupTables()
	upgradeTables()
	return nil
}

// Disconnect closes the database connection
// called only at server shutdown
func Disconnect() {
	log.Infow("shutdown", "message", "cleanly disconnected from database")
	if err := db.Close(); err != nil {
		log.Error(err)
	}
}

// setupTables checks for the existence of tables and creates them if needed
func setupTables() {
	var t = []struct {
		tablename string
		creation  string
	}{
		// agent must come first, team must come second, operation must come third, task fourth, the rest can be in alphabetical order
		{"agent", `CREATE TABLE agent ( gid char(21) NOT NULL, OneTimeToken varchar(64) NOT NULL DEFAULT uuid(), RISC tinyint(1) NOT NULL DEFAULT 0, intelname varchar(64) NOT NULL DEFAULT '', intelfaction tinyint(1) NOT NULL DEFAULT -1, picurl text DEFAULT NULL, PRIMARY KEY (gid), UNIQUE KEY OneTimeToken (OneTimeToken)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"team", `CREATE TABLE team ( teamID varchar(64) NOT NULL, owner char(21) NOT NULL, name varchar(64) DEFAULT NULL, rockskey varchar(32) DEFAULT NULL, rockscomm varchar(32) DEFAULT NULL, joinLinkToken varchar(64) DEFAULT NULL, vteam int(11) unsigned DEFAULT 0, vrole int(11) unsigned DEFAULT 0, PRIMARY KEY (teamID), KEY fk_team_owner (owner), CONSTRAINT fk_team_owner FOREIGN KEY (owner) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"operation", `CREATE TABLE operation ( ID char(40) NOT NULL, name varchar(128) NOT NULL DEFAULT 'new op', gid char(21) NOT NULL, color varchar(16) NOT NULL DEFAULT 'purple', modified datetime NOT NULL DEFAULT current_timestamp(), comment text DEFAULT NULL, referencetime datetime NOT NULL DEFAULT current_timestamp(), lasteditid char(40) NOT NULL DEFAULT 'unset', PRIMARY KEY (ID), KEY gid (gid), CONSTRAINT fk_operation_agent FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"task", `CREATE TABLE task(ID char(40) NOT NULL, opID char(40) NOT NULL, comment text DEFAULT NULL, taskorder int(11) NOT NULL DEFAULT 0, state enum('pending','assigned','acknowledged','completed') NOT NULL DEFAULT 'pending', zone tinyint(4) NOT NULL DEFAULT 1, delta int(11) NOT NULL DEFAULT 0, PRIMARY KEY (ID,opID), KEY fk_operation_id_task (opID), CONSTRAINT fk_operation_id_task FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},

		{"agentteams", `CREATE TABLE agentteams ( teamID varchar(64) NOT NULL, gid char(21) NOT NULL, shareLoc tinyint(4) NOT NULL DEFAULT 0, shareWD tinyint(4) NOT NULL DEFAULT 0, loadWD tinyint(4) NOT NULL DEFAULT 0, comment varchar(32) NOT NULL DEFAULT 'agents', PRIMARY KEY (teamID,gid), KEY gidkey (gid), CONSTRAINT fk_agent_teams FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE, CONSTRAINT fk_t_teams FOREIGN KEY (teamID) REFERENCES team (teamID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"assignments", `CREATE TABLE assignments ( opID char(40) NOT NULL, taskID char(40) NOT NULL, gid char(21) NOT NULL, KEY tid (opID,taskID), KEY opID (opID), KEY gid (gid), CONSTRAINT fk_assignments_gid FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE, CONSTRAINT fk_taskid FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"defensivekeys", `CREATE TABLE defensivekeys ( gid char(21) NOT NULL, portalID varchar(41) NOT NULL, capID varchar(16) DEFAULT NULL, count int(3) NOT NULL DEFAULT 0, name varchar(128) DEFAULT NULL, loc point DEFAULT NULL, PRIMARY KEY (portalID,gid), KEY fk_dk_gid (gid), CONSTRAINT fk_dk_gid FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"deletedops", `CREATE TABLE deletedops ( opID char(40) NOT NULL, deletedate datetime NOT NULL DEFAULT current_timestamp(), gid char(21) DEFAULT NULL, PRIMARY KEY (opID)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"depends", `CREATE TABLE depends ( opID char(40) NOT NULL, taskID char(40) NOT NULL, dependsOn char(40) DEFAULT NULL, KEY fk_depends_opID (opID), KEY key_optask (opID,taskID), CONSTRAINT fk_depends_opID FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"firebase", `CREATE TABLE firebase ( gid char(21) NOT NULL, token varchar(256) NOT NULL, KEY fk_gid (gid), CONSTRAINT fk_gid FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},

		{"link", `CREATE TABLE link (ID char(40) NOT NULL, opID char(40) NOT NULL, fromPortalID varchar(41) NOT NULL, toPortalID varchar(41) NOT NULL, color varchar(16) NOT NULL DEFAULT 'main', mu bigint(20) unsigned NOT NULL DEFAULT 0, PRIMARY KEY (ID,opID), KEY fk_operation_id_link (opID), KEY fk_task_link (ID) , CONSTRAINT fk_operation_id_link FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE, CONSTRAINT fk_task_link FOREIGN KEY (ID) REFERENCES task (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"locations", `CREATE TABLE locations ( gid char(21) NOT NULL, upTime datetime NOT NULL DEFAULT current_timestamp(), loc point NOT NULL, PRIMARY KEY (gid)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"marker", `CREATE TABLE marker (ID char(40) NOT NULL, opID char(40) NOT NULL, portalID varchar(41) NOT NULL, type varchar(24) NOT NULL, PRIMARY KEY (ID,opID), KEY fk_operation_marker (opID), CONSTRAINT fk_operation_marker FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE, KEY fk_task_marker (ID), CONSTRAINT fk_task_marker FOREIGN KEY (ID) REFERENCES task (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"markerattributes", `CREATE TABLE markerattributes ( ID char(40) NOT NULL, opID char(40) NOT NULL, markerID char(40) NOT NULL, assignedTo char(21) DEFAULT NULL, name varchar(32) NOT NULL DEFAULT 'unset', value text DEFAULT NULL, PRIMARY KEY (ID,opID), KEY fk_makerattr_opID (opID), KEY fk_marker_attr (markerID), KEY fk_agent_markerattr (assignedTo), CONSTRAINT fk_agent_markerattr FOREIGN KEY (assignedTo) REFERENCES agent (gid) ON DELETE CASCADE, CONSTRAINT fk_markerattr_opID FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE, KEY fk_marker_markerattr (ID), CONSTRAINT fk_marker_markerattr FOREIGN KEY (ID) REFERENCES marker (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"messagelog", `CREATE TABLE messagelog ( timestamp datetime NOT NULL DEFAULT current_timestamp(), gid char(21) NOT NULL, message text NOT NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"opkeys", `CREATE TABLE opkeys ( opID char(40) NOT NULL, portalID varchar(41) NOT NULL, gid char(21) NOT NULL, onhand int(11) unsigned NOT NULL DEFAULT 0, capsule varchar(16) DEFAULT NULL, UNIQUE KEY key_unique (opID,portalID,gid,capsule), KEY fk_operation_id_keys (opID), CONSTRAINT fk_operation_id_keys FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE, KEY fk_agent_keys (gid), CONSTRAINT fk_agent_keys FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"permissions", `CREATE TABLE permissions (teamID varchar(64) NOT NULL, opID char(40) NOT NULL, permission enum('read','write','assignedonly') NOT NULL DEFAULT 'read', zone tinyint(4) NOT NULL DEFAULT 0, KEY opID (opID), KEY teamID (teamID), CONSTRAINT fk_ops_teamID FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE, CONSTRAINT fk_teamIDs_op FOREIGN KEY (teamID) REFERENCES team (teamID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"portal", `CREATE TABLE portal ( ID varchar(41) NOT NULL, opID char(40) NOT NULL, name varchar(128) NOT NULL, loc point NOT NULL, comment text DEFAULT NULL, hardness varchar(64) DEFAULT NULL, PRIMARY KEY (ID,opID), KEY fk_operation_id (opID)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"rocks", `CREATE TABLE rocks ( gid char(21) NOT NULL, tgid int(11) DEFAULT NULL, agent varchar(64) DEFAULT NULL, verified tinyint(4) NOT NULL DEFAULT 0, smurf tinyint(4) NOT NULL DEFAULT 0, fetched datetime NOT NULL DEFAULT current_timestamp(), PRIMARY KEY (gid), CONSTRAINT fk_rocks_gid FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"telegram", `CREATE TABLE telegram (telegramID bigint(20) NOT NULL, telegramName varchar(32) NOT NULL, gid char(21) NOT NULL, verified tinyint(1) NOT NULL DEFAULT 0, authtoken varchar(32) DEFAULT NULL, PRIMARY KEY (telegramID), UNIQUE KEY gid (gid), CONSTRAINT fk_agent_telegram FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"telegramteam", `CREATE TABLE telegramteam (teamID varchar(64) NOT NULL, telegram bigint(20) NOT NULL, opID char(40) DEFAULT NULL, PRIMARY KEY (teamID,telegram), KEY (teamID), CONSTRAINT fk_tt_team FOREIGN KEY (teamID) REFERENCES team (teamID) ON DELETE CASCADE, UNIQUE KEY (telegram), KEY (opID), CONSTRAINT fk_tt_op FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"v", `CREATE TABLE v (gid char(21) NOT NULL, enlid char(42) NOT NULL, vlevel int(11) DEFAULT NULL, vpoints int(11) DEFAULT NULL, agent varchar(64) DEFAULT NULL, level int(11) DEFAULT NULL, quarantine tinyint(4) NOT NULL DEFAULT 0, active tinyint(4) NOT NULL DEFAULT 0, blacklisted tinyint(4) NOT NULL DEFAULT 0, verified tinyint(4) NOT NULL DEFAULT 0, flagged tinyint(4) NOT NULL DEFAULT 0, banned tinyint(4) NOT NULL DEFAULT 0, cellid varchar(32) DEFAULT NULL, telegram varchar(32) DEFAULT NULL, telegramID int(11) NOT NULL DEFAULT 0, startlat float DEFAULT NULL, startlon float DEFAULT NULL, distance int(11) DEFAULT NULL, vapikey char(42) DEFAULT NULL, fetched datetime NOT NULL DEFAULT utc_timestamp(), PRIMARY KEY (gid), CONSTRAINT fk_v_gid FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"zone", `CREATE TABLE zone (ID tinyint(4) NOT NULL, opID char(40) NOT NULL, name varchar(64) NOT NULL DEFAULT 'zone', color varchar(10) NOT NULL DEFAULT 'green', PRIMARY KEY (ID,opID), KEY fk_operation_zone (opID), CONSTRAINT fk_operation_zone FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"zonepoints", `CREATE TABLE zonepoints ( zoneID tinyint(4) NOT NULL, opID char(40) NOT NULL, position tinyint(4) NOT NULL, point point NOT NULL, PRIMARY KEY (zoneID,opID,position), KEY fk_operation_zonepoint (opID), CONSTRAINT fk_operation_zonepoint FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
	}

	var table string
	// use a tranaction to AVOID concurrency in this logic
	// it is possible for these to go in out-of-order and fk problems to show up under rare circumstances
	tx, err := db.Begin()
	if err != nil {
		log.Error(err)
		panic(err)
	}
	_, err = tx.Exec("SET FOREIGN_KEY_CHECKS=0")
	if err != nil {
		log.Error(err)
	}

	defer func() {
		err := tx.Rollback()
		if err != nil && err != sql.ErrTxDone {
			log.Error(err)
		}
		// tx is complete, use db
		_, err = db.Exec("SET FOREIGN_KEY_CHECKS=1")
		if err != nil {
			log.Error(err)
		}
	}()
	for _, v := range t {
		q := fmt.Sprintf("SHOW TABLES LIKE '%s'", v.tablename)
		err = tx.QueryRow(q).Scan(&table)
		if err != nil && err != sql.ErrNoRows {
			log.Error(err)
			continue
		}
		if err == sql.ErrNoRows || table == "" {
			log.Info("Setting up table:", v.tablename)
			_, err = tx.Exec(v.creation)
			if err != nil {
				log.Error(err)
			}
		}
	}
	_, err = tx.Exec("SET FOREIGN_KEY_CHECKS=1")
	if err != nil {
		log.Error(err)
	}
	err = tx.Commit() // the defer'd rollback will not have anything to rollback...
	if err != nil {
		log.Error(err)
	}
	// defer'd func runs here
}

func upgradeTables() {
	var upgrades = []struct {
		test    string // a query that will fail if an upgrade is needed
		upgrade string // the query to run to make the upgrade
	}{
		// none for now
	}

	tx, err := db.Begin()
	if err != nil {
		log.Error(err)
		panic(err)
	}
	_, err = tx.Exec("SET FOREIGN_KEY_CHECKS=0")
	if err != nil {
		log.Error(err)
	}
	defer func() {
		err := tx.Rollback()
		if err != nil && err != sql.ErrTxDone {
			log.Error(err)
		}
		_, err = db.Exec("SET FOREIGN_KEY_CHECKS=1")
		if err != nil {
			log.Error(err)
		}
	}()

	// do upgrades
	var scratch string
	for _, q := range upgrades {
		err = tx.QueryRow(q.test).Scan(&scratch)
		if err == nil {
			continue
		}
		log.Debugw("schema check failed", "test", q.test, "error", err.Error(), "doing upgrade", q.upgrade)
		_, err = tx.Exec(q.upgrade)
		if err != nil {
			log.Error(err)
			panic(err)
		}
	}

	// all upgrades done...
	_, err = tx.Exec("SET FOREIGN_KEY_CHECKS=1")
	if err != nil {
		log.Error(err)
	}
	err = tx.Commit()
	if err != nil {
		log.Error(err)
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
