package model

import (
	"fmt"
	"strconv"
	"time"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/v"
)

func init() {
	v.Callbacks.Team = vTeam
	v.Callbacks.FromDB = vFromDB
	v.Callbacks.ToDB = vToDB
}

// vToDB updates the database to reflect an agent's current status at V.
// callback
func vToDB(a v.Agent) error {
	if a.Agent == "" {
		return nil
	}
	_, err := db.Exec("REPLACE INTO v (enlid, gid, vlevel, vpoints, agent, level, quarantine, active, blacklisted, verified, flagged, banned, cellid, telegram, startlat, startlon, distance, fetched) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,UTC_TIMESTAMP())",
		a.EnlID, a.Gid, a.Vlevel, a.Vpoints, a.Agent, a.Level, a.Quarantine, a.Active, a.Blacklisted, a.Verified, a.Flagged, a.Banned, a.Cellid, a.TelegramID, a.StartLat, a.StartLon, a.Distance)
	if err != nil {
		log.Error(err)
		return err
	}

	// we trust .v to verify telegram info; if it is not already set for a agent, just import it.
	tgid, err := strconv.ParseInt(a.TelegramID, 10, 64)
	if err != nil {
		log.Error(err)
		return nil // not a deal-breaker
	}
	if tgid > 0 { // negative numbers are group chats, 0 is invalid
		if _, err := db.Exec("INSERT IGNORE INTO telegram (telegramID, telegramName, gid, verified) VALUES (?, ?, ?, 1)", tgid, a.Telegram, a.Gid); err != nil {
			log.Error(err)
			return err
		}
	}
	return nil
}

// callback
func vFromDB(g v.GoogleID) (v.Agent, time.Time, error) {
	gid := GoogleID(g)

	var a v.Agent
	var fetched string
	var t time.Time

	err := db.QueryRow("SELECT enlid, vlevel, vpoints, agent, level, quarantine, active, blacklisted, verified, flagged, banned, cellid, telegram, startlat, startlon, distance, fetched FROM v WHERE gid = ?", gid).Scan(&a.EnlID, &a.Gid, &a.Vlevel, &a.Vpoints, &a.Agent, &a.Level, &a.Quarantine, &a.Active, &a.Blacklisted, &a.Verified, &a.Flagged, &a.Banned, &a.Cellid, &a.TelegramID, &a.StartLat, &a.StartLon, &a.Distance, &fetched)
	if err != nil {
		return a, t, err
	}

	t, err = time.ParseInLocation("2006-01-02 15:04:05", fetched, time.UTC)
	if err != nil {
		return a, t, err
	}
	return a, t, nil
}

// callback
func vTeam(teamID v.TeamID) (int64, uint8, error) {
	var team int64
	var role uint8
	err := db.QueryRow("SELECT vteam, vrole FROM team WHERE teamID = ?", teamID).Scan(&team, &role)
	if err != nil {
		log.Error(err)
		return 0, 0, err
	}
	return team, role, nil
}

func getTeamsByVID(v int64) ([]TeamID, error) {
	var teams []TeamID

	row, err := db.Query("SELECT teamID FROM team WHERE vteam = ?", v)
	if err != nil {
		log.Error(err)
		return teams, err
	}
	defer row.Close()

	var teamID TeamID
	for row.Next() {
		err = row.Scan(&teamID)
		if err != nil {
			log.Error(err)
			continue
		}
		teams = append(teams, teamID)
	}
	return teams, nil
}

// VConfigure sets V connection for a Wasabee team -- caller should verify ownership
func (teamID TeamID) VConfigure(vteam int64, role uint8) error {
	_, ok := v.Roles[role]
	if !ok {
		err := fmt.Errorf("invalid role")
		log.Error(err)
		return err
	}

	log.Infow("linking team to V", "teamID", teamID, "vteam", vteam, "role", role)

	_, err := db.Exec("UPDATE team SET vteam = ?, vrole = ? WHERE teamID = ?", vteam, role, teamID)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}
