package main

import (
	"database/sql"

	"github.com/wasabee-project/Wasabee-Server/log"
)

func doDKeys() {
	rows, err := old.Query("SELECT gid, portalID, capID, count, name, loc FROM defensivekeys ORDER BY gid, name")
	if err != nil {
		log.Panic(err)
	}

	for rows.Next() {
		var gid, portalID string
		var count uint16
		var cap, name, loc sql.NullString

		err = rows.Scan(&gid, &portalID, &cap, &count, &name, &loc)
		if err != nil {
			log.Panic(err)
		}
		_, err = new.Exec("REPLACE INTO defensivekeys (gid, portalID, capID, count, name, loc) VALUES (?,?,?,?,?,?)", gid, portalID, cap, count, name, loc)
		if err != nil {
			log.Panic(err)
		}
	}
	countResults("defensivekeys", "defensivekeys")
}
