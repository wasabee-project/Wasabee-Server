package model

import (
	"database/sql"
	"time"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/rocks"
)

// setup the callbacks -- work around circular dependency problems
func init() {
	rocks.Callbacks.ToDB = rocksToDB
	rocks.Callbacks.FromDB = rocksFromDB
	rocks.Callbacks.AddAgentToTeam = rocksAddAgentToTeam
	rocks.Callbacks.RemoveAgentFromTeam = rocksRemoveAgentFromTeam
}

func rocksToDB(a rocks.Agent) error {
	if a.Agent == "" {
		return nil
	}
	_, err := db.Exec("REPLACE INTO rocks (gid, tgid, agent, verified, smurf, fetched) VALUES (?,?,?,?,?,UTC_TIMESTAMP())", a.Gid, a.TGId, a.Agent, a.Verified, a.Smurf)
	if err != nil {
		log.Error(err)
		return err
	}

	// we trust .rocks to verify telegram info; if it is not already set for a agent, just import it.
	if a.TGId > 0 { // negative numbers are group chats, 0 is invalid
		if _, err := db.Exec("INSERT IGNORE INTO telegram (telegramID, gid, verified) VALUES (?, ?, 1)", a.TGId, a.Gid); err != nil {
			log.Error(err)
			return err
		}
	}
	return nil
}

func rocksFromDB(g rocks.GoogleID) (rocks.Agent, time.Time, error) {
	var a rocks.Agent
	var t time.Time
	return a, t, nil
}

// RocksCommunity returns a rocks key for a TeamID
func (teamID TeamID) RocksCommunity() (string, error) {
	var rc sql.NullString
	err := db.QueryRow("SELECT rockskey FROM team WHERE teamID = ?", teamID).Scan(&rc)
	if err != nil {
		log.Error(err)
		return "", err
	}
	if !rc.Valid {
		return "", nil
	}
	return rc.String, nil
}

// RocksCommunityToTeam returns a TeamID from a Rocks Community
func RocksCommunityToTeam(communityID string) (TeamID, error) {
	var teamID TeamID
	err := db.QueryRow("SELECT teamID FROM team WHERE rockskey = ?", communityID).Scan(&teamID)
	if err != nil {
		log.Error(err)
		return "", err
	}
	return teamID, nil
}

func rocksAddAgentToTeam(g, communityID string) error {
	gid := GoogleID(g)

	if !gid.Valid() {
		log.Infow("Importing previously unknown agent", "GID", gid)
		err := gid.FirstLogin()
		if err != nil {
			log.Info(err)
			return err
		}
	}

	teamID, err := RocksCommunityToTeam(communityID)
	if err != nil {
		log.Error(err)
		return err
	}
	if err := teamID.AddAgent(gid); err != nil {
		log.Error(err)
		return err
	}

	return nil
}

func rocksRemoveAgentFromTeam(g, communityID string) error {
	gid := GoogleID(g)

	teamID, err := RocksCommunityToTeam(communityID)
	if err != nil {
		log.Error(err)
		return err
	}
	if err := teamID.RemoveAgent(gid); err != nil {
		log.Error(err)
		return err
	}

	return nil
}
