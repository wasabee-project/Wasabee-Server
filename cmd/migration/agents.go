package main

import (
	"database/sql"

	"github.com/wasabee-project/Wasabee-Server/log"
)

func doAgents() {
	rows, err := old.Query("select agent.gid, level, VVerified, Vblacklisted, Vid, RocksVerified, RISC, OneTimeToken, intelname, intelfaction, LEFT(Vname, 16), LEFT(rocksname, 16), telegramID, telegramName, verified, picurl, VAPIkey FROM agent LEFT JOIN agentextras ON agent.gid = agentextras.gid LEFT JOIN telegram ON agent.gid = telegram.gid ORDER BY agent.gid")
	if err != nil {
		log.Panic(err)
	}

	for rows.Next() {
		var gid, vid, ott, intelname, vname, rocksname, telegramName, picurl, vapikey sql.NullString
		var level, vverified, vblacklisted, rocksverified, risc, intelfaction, telegramID, tgverified sql.NullInt64

		err = rows.Scan(&gid, &level, &vverified, &vblacklisted, &vid, &rocksverified, &risc, &ott, &intelname, &intelfaction, &vname, &rocksname, &telegramID, &telegramName, &tgverified, &picurl, &vapikey)
		if err != nil {
			log.Panic(err)
		}

		_, err := new.Exec("REPLACE INTO agent (gid, OneTimeToken, RISC, intelname, intelfaction, picurl) VALUES (?, ?, ?, ?, ?, ?)", gid, ott, risc, intelname, intelfaction, picurl)
		if err != nil {
			log.Panic(err)
		}

		_, err = new.Exec("REPLACE INTO locations (gid, loc) VALUES (?, POINT(0,0))", gid)
		if err != nil {
			log.Panic(err)
		}

		if telegramID.Valid && tgverified.Int64 != 0 {
			if telegramName.String == "unused" {
				telegramName.String = ""
			}
			_, err = new.Exec("REPLACE INTO telegram (telegramID, telegramName, gid, verified) VALUES (?, ?, ?, ?)", telegramID, telegramName, gid, tgverified)
			if err != nil {
				log.Panic(err)
			}
		}

		if rocksname.Valid && rocksname.String != "" {
			_, err = new.Exec("REPLACE INTO rocks (gid, agent, verified) VALUES (?, ?, ?)", gid, rocksname, rocksverified)
			if err != nil {
				log.Panic(err)
			}
		}

		if vname.Valid && vid.Valid && vname.String != "" {
			_, err = new.Exec("REPLACE INTO v (gid, enlid, agent, level, blacklisted, verified) VALUES (?, ?, ?, ?, ?, ?)", gid, vid, vname, level, vblacklisted, vverified)
			if err != nil {
				log.Panic(err)
			}
		}
	}
	rows.Close()
	countResults("agent", "agent")

	rows, err = old.Query("SELECT gid, token FROM firebase ORDER BY gid")
	if err != nil {
		log.Panic(err)
	}

	for rows.Next() {
		var gid, token string

		rows.Scan(&gid, &token)

		_, err := new.Exec("REPLACE INTO firebase (gid, token) VALUES (?, ?)", gid, token)
		if err != nil {
			log.Debug("problem", "gid", gid, "token", token)
			log.Panic(err)
		}
	}
	countResults("firebase", "firebase")
}
