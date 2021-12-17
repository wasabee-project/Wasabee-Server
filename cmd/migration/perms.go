package main

import (
	"database/sql"

	"github.com/wasabee-project/Wasabee-Server/log"
)

func doPermissions() {
	rows, err := old.Query("SELECT teamID, opID, permission, zone FROM opteams ORDER BY teamID, opID")
	if err != nil {
		log.Panic(err)
	}

	for rows.Next() {
		var teamID, opID, permission string
		var zone sql.NullInt64

		err = rows.Scan(&teamID, &opID, &permission, &zone)
		if err != nil {
			log.Panic(err)
		}
		_, err = new.Exec("INSERT INTO permissions (teamID, opID, permission, zone) VALUES (?,?,?,?)", teamID, opID, permission, zone)
		if err != nil {
			log.Panic(err)
		}
	}
	countResults("opteams", "permissions")
}
