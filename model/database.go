package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	// need a comment here to make lint happy
	_ "github.com/go-sql-driver/mysql"
	"github.com/wasabee-project/Wasabee-Server/log"
)

// db is the private global used by all relevant functions to interact with the database
var db *sql.DB

// Connect tries to establish a connection to a MySQL/MariaDB database
func Connect(ctx context.Context, uri string) error {
	result, err := sql.Open("mysql", uri)
	if err != nil {
		log.Error(err)
		return err
	}
	db = result

	var version string
	// Using QueryRowContext to ensure the initial ping/check respects startup timeout
	if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
		log.Error(err)
		return err
	}
	log.Infow("startup", "database", "connected", "version", version, "message", "connected to database")

	setupTables(ctx)
	upgradeTables(ctx)
	optimizeTables(ctx)
	return nil
}

// Disconnect closes the database connection
func Disconnect() {
	log.Infow("shutdown", "message", "cleanly disconnected from database")
	if db != nil {
		if err := db.Close(); err != nil {
			log.Error(err)
		}
	}
}

var tabledefs = []struct {
	tablename string
	creation  string
}{
	{"agent", `CREATE TABLE agent (gid char(21) NOT NULL, OneTimeToken varchar(64) NOT NULL DEFAULT "", RISC tinyint(1) NOT NULL DEFAULT 0, intelname varchar(16) DEFAULT NULL, intelfaction tinyint(1) NOT NULL DEFAULT -1, communityname varchar(16) DEFAULT NULL, picurl text DEFAULT NULL, PRIMARY KEY (gid), UNIQUE KEY OneTimeToken (OneTimeToken), UNIQUE KEY communityname (communityname)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
	{"team", `CREATE TABLE team (teamID varchar(64) NOT NULL, owner char(21) NOT NULL, name varchar(64) DEFAULT NULL, rockskey varchar(32) DEFAULT NULL, rockscomm varchar(32) DEFAULT NULL, joinLinkToken varchar(64) DEFAULT NULL, vteam int(11) unsigned DEFAULT 0, vrole int(11) unsigned DEFAULT 0, PRIMARY KEY (teamID), KEY fk_team_owner (owner), CONSTRAINT fk_team_owner FOREIGN KEY (owner) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
	{"operation", `CREATE TABLE operation (ID char(40) NOT NULL, name varchar(128) NOT NULL DEFAULT 'new op', gid char(21) NOT NULL, color varchar(16) NOT NULL DEFAULT 'purple', modified timestamp NOT NULL DEFAULT current_timestamp(), comment text DEFAULT NULL, referencetime timestamp NOT NULL DEFAULT current_timestamp(), lasteditid char(40) NOT NULL DEFAULT 'unset', PRIMARY KEY (ID), KEY gid (gid), CONSTRAINT fk_operation_agent FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
	{"task", `CREATE TABLE task(ID char(40) NOT NULL, opID char(40) NOT NULL, comment text DEFAULT NULL, taskorder int(11) NOT NULL DEFAULT 0, state enum('pending','assigned','acknowledged','completed') NOT NULL DEFAULT 'pending', zone tinyint(4) NOT NULL DEFAULT 1, delta int(11) NOT NULL DEFAULT 0, PRIMARY KEY (ID,opID), KEY fk_operation_id_task (opID), CONSTRAINT fk_operation_id_task FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
	{"agentteams", `CREATE TABLE agentteams (teamID varchar(64) NOT NULL, gid char(21) NOT NULL, shareLoc tinyint(4) NOT NULL DEFAULT 0, shareWD tinyint(4) NOT NULL DEFAULT 0, loadWD tinyint(4) NOT NULL DEFAULT 0, comment varchar(32), PRIMARY KEY (teamID,gid), KEY gidkey (gid), CONSTRAINT fk_agent_teams FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE, CONSTRAINT fk_t_teams FOREIGN KEY (teamID) REFERENCES team (teamID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
	{"assignments", `CREATE TABLE assignments (opID char(40) NOT NULL, taskID char(40) NOT NULL, gid char(21) NOT NULL, KEY opID (opID), KEY gid (gid), CONSTRAINT fk_assignments_gid FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE, CONSTRAINT fk_assignments_opid FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE, KEY taskID (taskID,opID), CONSTRAINT fk_assignments_taskid FOREIGN KEY (taskID,opID) REFERENCES task (ID, opID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
	{"defensivekeys", `CREATE TABLE defensivekeys (gid char(21) NOT NULL, portalID varchar(41) NOT NULL, capID varchar(16) DEFAULT NULL, count int(3) NOT NULL DEFAULT 0, name varchar(128) DEFAULT NULL, loc point DEFAULT NULL, PRIMARY KEY (portalID,gid), KEY fk_dk_gid (gid), CONSTRAINT fk_dk_gid FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
	{"deletedops", `CREATE TABLE deletedops (opID char(40) NOT NULL, deletedate timestamp NOT NULL DEFAULT current_timestamp(), gid char(21) DEFAULT NULL, PRIMARY KEY (opID)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
	{"depends", `CREATE TABLE depends (opID char(40) NOT NULL, taskID char(40) NOT NULL, dependsOn char(40) DEFAULT NULL, KEY fk_depends_opID (opID), PRIMARY KEY key_optask (opID,taskID), CONSTRAINT fk_depends_opID FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE, CONSTRAINT fk_depends_pk FOREIGN KEY (taskID, opID) REFERENCES task (ID, opID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
	{"firebase", `CREATE TABLE firebase (gid char(21) NOT NULL, token varchar(256) NOT NULL, KEY fk_gid (gid), CONSTRAINT fk_gid FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
	{"link", `CREATE TABLE link (ID char(40) NOT NULL, opID char(40) NOT NULL, fromPortalID varchar(41) NOT NULL, toPortalID varchar(41) NOT NULL, color varchar(16) NOT NULL DEFAULT 'main', mu bigint(20) unsigned NOT NULL DEFAULT 0, PRIMARY KEY (ID,opID), KEY fk_operation_id_link (opID), KEY fk_task_link (ID) , CONSTRAINT fk_operation_id_link FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE, CONSTRAINT fk_task_link FOREIGN KEY (ID) REFERENCES task (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
	{"locations", `CREATE TABLE locations (gid char(21) NOT NULL, upTime timestamp NOT NULL DEFAULT current_timestamp(), loc point NOT NULL, PRIMARY KEY (gid), CONSTRAINT fk_location_agent FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
	{"marker", `CREATE TABLE marker (ID char(40) NOT NULL, opID char(40) NOT NULL, portalID varchar(41) NOT NULL, type varchar(24) NOT NULL, PRIMARY KEY (ID,opID), KEY fk_operation_marker (opID), CONSTRAINT fk_operation_marker FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE, KEY fk_task_marker (ID), CONSTRAINT fk_task_marker FOREIGN KEY (ID) REFERENCES task (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
	{"markerattributes", `CREATE TABLE markerattributes (ID char(40) NOT NULL, opID char(40) NOT NULL, markerID char(40) NOT NULL, name varchar(32) NOT NULL DEFAULT 'unset', value text DEFAULT NULL, PRIMARY KEY (ID,opID), KEY fk_makerattr_opID (opID), KEY fk_marker_attr (markerID), CONSTRAINT fk_markerattr_opID FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE, KEY fk_marker_markerattr (ID), CONSTRAINT fk_marker_markerattr FOREIGN KEY (ID) REFERENCES marker (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
	{"messagelog", `CREATE TABLE messagelog (timestamp timestamp NOT NULL DEFAULT current_timestamp(), gid char(21) NOT NULL, message text NOT NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
	{"opkeys", `CREATE TABLE opkeys (opID char(40) NOT NULL, portalID varchar(41) NOT NULL, gid char(21) NOT NULL, onhand int(11) unsigned NOT NULL DEFAULT 0, capsule varchar(16) DEFAULT NULL, UNIQUE KEY key_unique (opID,portalID,gid,capsule), KEY fk_operation_id_keys (opID), CONSTRAINT fk_operation_id_keys FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE, KEY fk_agent_keys (gid), CONSTRAINT fk_agent_keys FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
	{"permissions", `CREATE TABLE permissions (teamID varchar(64) NOT NULL, opID char(40) NOT NULL, permission enum('read','write','assignedonly') NOT NULL DEFAULT 'read', zone tinyint(4) NOT NULL DEFAULT 0, KEY opID (opID), KEY teamID (teamID), CONSTRAINT fk_ops_teamID FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE, CONSTRAINT fk_teamIDs_op FOREIGN KEY (teamID) REFERENCES team (teamID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
	{"portal", `CREATE TABLE portal (ID varchar(41) NOT NULL, opID char(40) NOT NULL, name varchar(128) NOT NULL, loc point NOT NULL, comment text DEFAULT NULL, hardness varchar(64) DEFAULT NULL, PRIMARY KEY (ID,opID), KEY fk_operation_id (opID), CONSTRAINT FOREIGN KEY fk_operation_id (opID) REFERENCES operation (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
	{"rocks", `CREATE TABLE rocks (gid char(21) NOT NULL, tgid int(11) DEFAULT NULL, agent varchar(16) DEFAULT NULL, verified tinyint(4) NOT NULL DEFAULT 0, smurf tinyint(4) NOT NULL DEFAULT 0, fetched timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (gid), CONSTRAINT fk_rocks_gid FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
	{"telegram", `CREATE TABLE telegram (telegramID bigint(20) NOT NULL, telegramName varchar(32) NOT NULL, gid char(21) NOT NULL, verified tinyint(1) NOT NULL DEFAULT 0, authtoken varchar(32) DEFAULT NULL, PRIMARY KEY (telegramID), UNIQUE KEY gid (gid), CONSTRAINT fk_agent_telegram FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
	{"telegramteam", `CREATE TABLE telegramteam (teamID varchar(64) NOT NULL, telegram bigint(20) NOT NULL, opID char(40) DEFAULT NULL, PRIMARY KEY (telegram), UNIQUE KEY (teamID), CONSTRAINT fk_tt_team FOREIGN KEY (teamID) REFERENCES team (teamID) ON DELETE CASCADE, KEY (opID), CONSTRAINT fk_tt_op FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
	{"telegramchatmembers", `CREATE TABLE telegramchatmembers (agent bigint(20) NOT NULL, chat bigint(20) NOT NULL, PRIMARY KEY (agent,chat), KEY (chat), CONSTRAINT fk_tg_chat FOREIGN KEY (chat) REFERENCES telegramteam (telegram) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
	{"zone", `CREATE TABLE zone (ID tinyint(4) NOT NULL, opID char(40) NOT NULL, name varchar(64) NOT NULL DEFAULT 'zone', color varchar(10) NOT NULL DEFAULT 'green', PRIMARY KEY (ID,opID), KEY fk_operation_zone (opID), CONSTRAINT fk_operation_zone FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
	{"zonepoints", `CREATE TABLE zonepoints (zoneID tinyint(4) NOT NULL, opID char(40) NOT NULL, position tinyint(4) UNSIGNED NOT NULL, point point NOT NULL, PRIMARY KEY (zoneID,opID,position), KEY fk_operation_zonepoint (opID), CONSTRAINT fk_operation_zonepoint FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
}

func setupTables(ctx context.Context) {
	var table string
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Error(err)
		panic(err)
	}

	_, _ = tx.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0")

	defer func() {
		_ = tx.Rollback()
		// We use context.Background() here to ensure cleanup happens even if startup was canceled
		_, _ = db.ExecContext(context.Background(), "SET FOREIGN_KEY_CHECKS=1")
	}()

	for _, v := range tabledefs {
		q := fmt.Sprintf("SHOW TABLES LIKE '%s'", v.tablename)
		err = tx.QueryRowContext(ctx, q).Scan(&table)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			log.Error(err)
			continue
		}
		if errors.Is(err, sql.ErrNoRows) || table == "" {
			log.Info("Setting up table:", v.tablename)
			if _, err = tx.ExecContext(ctx, v.creation); err != nil {
				log.Error(err)
			}
		}
	}

	if err = tx.Commit(); err != nil {
		log.Error(err)
	}
}

func upgradeTables(ctx context.Context) {
	var upgrades = []struct {
		test    string
		upgrade string
	}{
		{"SHOW FIELDS FROM zonepoints where field='position' and type like '%unsigned%'", "alter table zonepoints MODIFY COLUMN position tinyint(4) unsigned"},
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Error(err)
		panic(err)
	}
	_, _ = tx.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0")

	defer func() {
		_ = tx.Rollback()
		_, _ = db.ExecContext(context.Background(), "SET FOREIGN_KEY_CHECKS=1")
	}()

	var scratch string
	for _, q := range upgrades {
		err = tx.QueryRowContext(ctx, q.test).Scan(&scratch)
		if err == nil {
			continue
		}
		log.Debugw("schema check failed", "test", q.test, "doing upgrade", q.upgrade)
		if _, err = tx.ExecContext(ctx, q.upgrade); err != nil {
			log.Error(err)
			panic(err)
		}
	}

	if err = tx.Commit(); err != nil {
		log.Error(err)
	}
}

func optimizeTables(ctx context.Context) {
	for _, table := range tabledefs {
		log.Debugw("optimizing table", "table", table.tablename)
		// Optimize table usually takes a while; context allows it to be interrupted
		if _, err := db.ExecContext(ctx, fmt.Sprintf("OPTIMIZE TABLE %s", table.tablename)); err != nil {
			log.Error(err)
		}
	}
}

func makeNullString(in interface{}) sql.NullString {
	var s string
	switch v := in.(type) {
	case string:
		s = v
	case fmt.Stringer:
		s = v.String()
	default:
		return sql.NullString{}
	}

	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{
		String: s,
		Valid:  true,
	}
}

// Helper to handle optional transactions
type dbExecutor interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

func txExecutor(tx *sql.Tx) dbExecutor {
	if tx != nil {
		return tx
	}
	return db
}
