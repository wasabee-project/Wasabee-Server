package model

import (
	"database/sql"
	"time"

	"github.com/wasabee-project/Wasabee-Server/log"
)

type RocksAgent struct {
	Gid      GoogleID `json:"gid"`
	TGId     int64    `json:"tgid"`
	Agent    string   `json:"agentid"`
	Verified bool     `json:"verified"`
	Smurf    bool     `json:"smurf"`
}

func RocksToDB(a *RocksAgent) error {
	if a.Agent == "" {
		return nil
	}

	if a.Gid == "" {
		log.Info("empty GID in agent", "agent", a)
		return nil
	}

	_, err := db.Exec("REPLACE INTO rocks (gid, tgid, agent, verified, smurf, fetched) VALUES (?,?,?,?,?,UTC_TIMESTAMP())", a.Gid, a.TGId, a.Agent, a.Verified, a.Smurf)
	if err != nil {
		// there is a race somewhere that tries to insert this here before the agent is in the agent table, just log it for now
		log.Error(err)
		return nil
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

func RocksFromDB(gid GoogleID) (*RocksAgent, time.Time, error) {
	var a RocksAgent
	var fetched string
	var t time.Time
	var tgid sql.NullInt64
	var agent sql.NullString

	err := db.QueryRow("SELECT gid, tgid, agent, verified, smurf, fetched FROM rocks WHERE gid = ?", gid).Scan(&a.Gid, &tgid, &agent, &a.Verified, &a.Smurf, &fetched)
	if err != nil && err != sql.ErrNoRows {
		log.Error(err)
		return &a, t, err
	}

	if err == sql.ErrNoRows {
		return &a, t, nil
	}

	if agent.Valid {
		a.Agent = agent.String
	}
	if tgid.Valid {
		a.TGId = tgid.Int64
	}

	if fetched == "" {
		return &a, t, nil
	}

	t, err = time.ParseInLocation("2006-01-02 15:04:05", fetched, time.UTC)
	if err != nil {
		log.Error(err)
		return &a, t, err
	}
	// log.Debugw("rocks from cache", "fetched", t, "data", a)

	return &a, t, nil
}

// RocksCommunity returns a communityID for a TeamID
func (teamID TeamID) RocksCommunity() (string, error) {
	var rc sql.NullString
	err := db.QueryRow("SELECT rockscomm FROM team WHERE teamID = ?", teamID).Scan(&rc)
	if err != nil {
		log.Error(err)
		return "", err
	}
	if !rc.Valid {
		return "", nil
	}
	return rc.String, nil
}

// RocksKey returns a rocks key for a TeamID
func (teamID TeamID) RocksKey() (string, error) {
	var rc sql.NullString
	err := db.QueryRow("SELECT rockskey FROM team WHERE teamID = ?", teamID).Scan(&rc)
	if err != nil && err != sql.ErrNoRows {
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
	err := db.QueryRow("SELECT teamID FROM team WHERE rockscomm = ?", communityID).Scan(&teamID)
	if err != nil {
		log.Errorw("rocks community team lookup", "error", err.Error(), "community", communityID)
		return "", err
	}
	return teamID, nil
}

func RocksAddAgentToTeam(g, communityID string) error {
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

func RocksRemoveAgentFromTeam(g, communityID string) error {
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
