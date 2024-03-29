package model

import (
	"database/sql"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
)

// GetFirebaseTokens gets an agents FirebaseToken from the database
func (gid GoogleID) GetFirebaseTokens() ([]string, error) {
	var token string
	var toks []string

	rows, err := db.Query("SELECT DISTINCT token FROM firebase WHERE gid = ?", gid)
	if err != nil && err != sql.ErrNoRows {
		log.Error(err)
		return toks, err
	}
	// this is technically redundant with the main return, but be explicit about what we want
	if err != nil && err == sql.ErrNoRows {
		return toks, nil
	}
	defer rows.Close()

	for rows.Next() {
		err := rows.Scan(&token)
		if err != nil {
			log.Error(err)
			continue
		}
		toks = append(toks, token)
	}

	return toks, nil
}

// StoreFirebaseToken adds a token in the database for an agent.
// gid is not unique, an agent may have any number of tokens (e.g. multiple devices/browsers).
// Pruning of dead tokens takes place in the senders upon error.
func (gid GoogleID) StoreFirebaseToken(token string) error {
	g := GoogleID(gid)

	var count int
	err := db.QueryRow("SELECT COUNT(gid) FROM firebase WHERE token = ? AND gid = ?", token, gid).Scan(&count)
	if err != nil {
		log.Error(err)
		return err
	}

	if count > 0 {
		return nil
	}

	// log.Debugw("adding token", "subsystem", "Firebase", "GID", gid, "token", token)
	_, err = db.Exec("INSERT INTO firebase (gid, token) VALUES (?, ?)", gid, token)
	if err != nil {
		log.Error(err)
		return err
	}

	// Subscribe this token to all team topics
	// TODO: This isn't right -- each token sub now triggers messages in Telegram teams...
	for _, teamID := range g.teamList() {
		messaging.AddToRemote(messaging.GoogleID(gid), messaging.TeamID(teamID))
	}

	return nil
}

// RemoveFirebaseToken removes a given token from the database
func RemoveFirebaseToken(token string) error {
	_, err := db.Exec("DELETE FROM firebase WHERE token = ?", token)
	if err != nil {
		log.Error(err)
	}
	return err
}

// RemoveAllFirebaseTokens removes all tokens for a given agent
func (gid GoogleID) RemoveAllFirebaseTokens() error {
	_, err := db.Exec("DELETE FROM firebase WHERE gid = ?", gid)
	if err != nil {
		log.Error(err)
	}
	return err
}

// FirebaseBroadcastList returns all known firebase tokens for messaging all agents
// Firebase Multicast messages are limited to 500 tokens each, the caller must
// break the list up if necessary.
func FirebaseBroadcastList() ([]string, error) {
	var out []string

	rows, err := db.Query("SELECT DISTINCT token FROM firebase")
	if err != nil && err == sql.ErrNoRows {
		return out, nil
	}
	if err != nil {
		log.Error(err)
		return out, err
	}
	defer rows.Close()

	for rows.Next() {
		var token string
		err := rows.Scan(&token)
		if err != nil {
			log.Error(err)
			continue
		}
		out = append(out, token)
	}
	return out, nil
}

// TeamToken is the returned struct from FirebaseLocationTokens
type TeamToken struct {
	TeamID TeamID
	Token  string
}

// FirebaserLocationTokens returns a list all tokens for the agents on the teams with which this agent is sharing location
// instead of sending to the team topics, we do the fanout manually -- to avoid hitting the (small) fanout quota
func (gid GoogleID) FirebaseLocationTokens() ([]TeamToken, error) {
	var out []TeamToken

	rows, err := db.Query("SELECT DISTINCT teamid, token FROM firebase JOIN agentteams ON firebase.gid = agentteams.gid WHERE agentteams.teamID IN (SELECT teamID FROM agentteams WHERE gid = ? AND shareLoc = 1)", gid)
	if err != nil && err == sql.ErrNoRows {
		return out, nil
	}
	if err != nil {
		log.Error(err)
		return out, err
	}
	defer rows.Close()

	for rows.Next() {
		var tt TeamToken
		if err := rows.Scan(&tt.TeamID, &tt.Token); err != nil {
			log.Error(err)
			continue
		}
		out = append(out, tt)
	}
	return out, nil
}
