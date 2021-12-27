package main

import (
	"database/sql"

	"github.com/wasabee-project/Wasabee-Server/log"
)

func doTeams() {
	rows, err := old.Query("SELECT teamID, owner, name, rockskey, rockscomm, joinLinkToken, telegram, vteam, vrole FROM team ORDER BY teamID")
	if err != nil {
		log.Panic(err)
	}

	for rows.Next() {
		var teamID, owner string
		var name, rockskey, rockscomm, joinlinktoken sql.NullString
		var vteam, vrole, telegram sql.NullInt64

		err = rows.Scan(&teamID, &owner, &name, &rockskey, &rockscomm, &joinlinktoken, &telegram, &vteam, &vrole)
		if err != nil {
			log.Panic(err)
		}
		_, err = new.Exec("REPLACE INTO team (teamID, owner, name, rockskey, rockscomm, joinLinkToken, vteam, vrole) VALUES (?,?,?,?,?,?,?,?)", teamID, owner, name, rockskey, rockscomm, joinlinktoken, vteam, vrole)
		if err != nil {
			log.Panic(err)
		}
		if telegram.Valid {
			_, err = new.Exec("REPLACE INTO telegramteam (teamID, telegram, opID) VALUES (?,?,NULL)", teamID, telegram)
			if err != nil {
				log.Panic(err)
			}
		}
	}
	countResults("team", "team")
}

func doTeamMembership() {
	rows, err := old.Query("SELECT teamID, gid, state, squad, shareWD, loadWD FROM agentteams ORDER BY teamID, gid")
	if err != nil {
		log.Panic(err)
	}

	for rows.Next() {
		var teamID, gid, loadwd, sharewd, state string
		var comment sql.NullString
		var loadwdB, sharewdB, stateB bool

		err = rows.Scan(&teamID, &gid, &state, &comment, &sharewd, &loadwd)
		if err != nil {
			log.Panic(err)
		}

		if !comment.Valid {
			comment.String = ""
			comment.Valid = true
		}
		if comment.String == "joined via link" || comment.String == "00FF00" {
			comment.String = ""
		}
		if state == "On" {
			stateB = true
		}
		if loadwd == "On" {
			loadwdB = true
		}
		if sharewd == "On" {
			sharewdB = true
		}

		_, err = new.Exec("REPLACE INTO agentteams (teamID, gid, shareLoc, comment, shareWD, loadWD) VALUES (?,?,?,?,?,?)", teamID, gid, stateB, comment, sharewdB, loadwdB)
		if err != nil {
			log.Panic(err)
		}
	}
	countResults("agentteams", "agentteams")
}
