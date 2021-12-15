package main

import (
	"database/sql"

	"github.com/wasabee-project/Wasabee-Server/log"
)

func doOperations() {
	rows, err := old.Query("SELECT ID, name, gid, color, modified, comment, lasteditid, referencetime FROM operation ORDER BY ID")
	if err != nil {
		log.Panic(err)
	}

	for rows.Next() {
		var id, name, gid, color, modified, lasteditid, referencetime  string
		var comment sql.NullString

		err := rows.Scan(&id, &name, &gid, &color, &modified, &comment, &lasteditid, &referencetime)
		if err != nil {
			log.Panic(err)
		}
		_, err = new.Exec("INSERT INTO operation (ID, name, gid, color, modified, comment, referencetime, lasteditid) VALUES (?,?,?,?,?,?,?,?)", id, name, gid, color, modified, comment, referencetime, lasteditid)
		if err != nil {
			log.Panic(err)
		}
	}
	rows.Close()

	countResults("operation", "operation")

	// portals
	rows, err = old.Query("SELECT ID, opID, name, loc, comment, hardness FROM portal ORDER BY opID, ID")
	if err != nil {
		log.Panic(err)
	}

	for rows.Next() {
		var id, opid, name, loc string
		var comment, hardness sql.NullString

		err := rows.Scan(&id, &opid, &name, &loc, &comment, &hardness)
		if err != nil {
			log.Panic(err)
		}
		_, err = new.Exec("INSERT INTO portal (ID, opID, name, loc, comment, hardness) VALUES (?,?,?,?,?,?)", id, opid, name, loc, comment, hardness)
		if err != nil {
			log.Errorw("err", "id", id, "opID", opid, "err", err.Error())
			continue
			// log.Panic(err)
		}
	}
	rows.Close()
	countResults("portal", "portal")

	// markers
	rows, err = old.Query("SELECT ID, opID, portalID, type, gid, comment, complete, state, oporder, zone, deltaminutes FROM marker ORDER BY opID, ID")
	if err != nil {
		log.Panic(err)
	}

	for rows.Next() {
		var id, opid, portalid, typeid string
		var taskorder int64
		var comment, state, completed, gid sql.NullString
		var zone, deltaminutes sql.NullInt64

		err := rows.Scan(&id, &opid, &portalid, &typeid, &gid, &comment, &completed, &state, &taskorder, &zone, &deltaminutes)
		if err != nil {
			log.Panic(err)
		}

		_, err = new.Exec("INSERT INTO task (ID, opID, comment, taskorder, state, zone, delta)  VALUES (?,?,?,?,?,?,?)", id, opid, comment, taskorder, state, zone, deltaminutes)
		if err != nil {
			log.Debug("out", "taskorder", taskorder, "state", state, "zone", zone, "delta", deltaminutes);
			log.Panic(err)
		}
		_, err = new.Exec("INSERT INTO marker (ID, opID, portalID, type) VALUES (?,?,?,?)", id, opid, portalid, typeid)
		if err != nil {
			log.Panic(err)
		}

		if gid.Valid {
			_, err = new.Exec("INSERT INTO assignments (taskID, opID, gid) VALUES (?,?,?)", id, opid, gid)
			if err != nil {
				log.Panic(err)
			}
		}
	}
	rows.Close()
	countResults("marker", "marker")

	// links
	rows, err = old.Query("SELECT ID, fromPortalID, toPortalID, opID, description, gid, throworder, completed, color, zone, deltaminutes, mu FROM link")
	if err != nil {
		log.Panic(err)
	}
	for rows.Next() {
		var id, opid, fromPortalID, toPortalID, state string
		var taskorder, completed int
		var comment, gid, color sql.NullString
		var zone, deltaminutes, mu sql.NullInt64


		state = "pending"
		if gid.Valid {
			state = "assigned"
		}
		if completed == 1 && gid.Valid {
			state = "completed"
		}

		err := rows.Scan(&id, &fromPortalID, &toPortalID, &opid, &comment, &gid, &taskorder, &completed, &color, &zone, &deltaminutes, &mu)
		if err != nil {
			log.Panic(err)
		}
		// log.Infow("link", "linkID", id, "from", fromPortalID, "to", toPortalID, "order", taskorder, "completed", completed)
		_, err = new.Exec("INSERT INTO task (ID, opID, comment, taskorder, state, zone, delta) VALUES (?,?,?,?,?,?,?)", id, opid, comment, taskorder, state, zone, deltaminutes)
		if err != nil {
			log.Panic(err)
		}
		_, err = new.Exec("INSERT INTO link (ID, opID, fromPortalID, toPortalID, color, mu)  VALUES (?,?,?,?,?,?)", id, opid, fromPortalID, toPortalID, color, mu)
		if err != nil {
			log.Panic(err)
		}

		if gid.Valid {
			_, err = new.Exec("INSERT INTO assignments (taskID, opID, gid) VALUES (?,?,?)", id, opid, gid)
			if err != nil {
				log.Panic(err)
			}
		}
	}
	rows.Close()
	countResults("link", "link")

	// zones
	rows, err = old.Query("SELECT ID, opID, name, color FROM zone ORDER BY opID, ID")
	if err != nil {
		log.Panic(err)
	}

	for rows.Next() {
		var opid, name, color string
		var id uint8

		err := rows.Scan(&id, &opid, &name, &color)
		if err != nil {
			log.Panic(err)
		}
		_, err = new.Exec("INSERT INTO zone (ID, opID, name, color) VALUES (?,?,?,?)", id, opid, name, color)
		if err != nil {
			log.Panic(err)
		}
	}
	rows.Close()
	countResults("zone", "zone")

	// zone points
	rows, err = old.Query("SELECT zoneID, opID, position, point FROM zonepoints ORDER BY opID, zoneID, position")
	if err != nil {
		log.Panic(err)
	}

	for rows.Next() {
		var opid, point string
		var id, position uint8

		err := rows.Scan(&id, &opid, &position, &point)
		if err != nil {
			log.Panic(err)
		}
		_, err = new.Exec("INSERT INTO zonepoints (zoneID, opID, position, point)  VALUES (?,?,?,?)", id, opid, position, point)
		if err != nil {
			log.Panic(err)
		}
	}
	rows.Close()
	countResults("zonepoints", "zonepoints")
}
