package model

import (
	"database/sql"

	"github.com/wasabee-project/Wasabee-Server/Firebase"
	"github.com/wasabee-project/Wasabee-Server/log"
)

// init sets up the callbacks used to get around the need for circular dependencies.
// this model depends on wfb but wfb needs a few things to get/store/delete tokens,
func init() {
	wfb.Callbacks.GidToTokens = GetFirebaseTokens
	wfb.Callbacks.StoreToken = StoreFirebaseToken
	wfb.Callbacks.DeleteToken = RemoveFirebaseToken
	wfb.Callbacks.BroadcastList = FirebaseBroadcastList
}

// FirebaseTokens gets an agents FirebaseToken from the database
func GetFirebaseTokens(gid wfb.GoogleID) ([]string, error) {
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
func StoreFirebaseToken(gid wfb.GoogleID, token string) error {
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

	log.Debugw("adding token", "subsystem", "Firebase", "GID", gid, "token", token)
	_, err = db.Exec("INSERT INTO firebase (gid, token) VALUES (?, ?)", gid, token)
	if err != nil {
		log.Error(err)
		return err
	}

	// Subscribe to all team topics
	tl := g.teamList()
	for _, teamID := range tl {
		wfb.SubscribeToTopic(wfb.GoogleID(gid), wfb.TeamID(teamID))
	}

	return nil
}

// pass this as a callback to Firebase so it can remove rejected tokens
func RemoveFirebaseToken(token string) error {
	_, err := db.Exec("DELETE FROM firebase WHERE token = ?", token)
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

	var token string
	for rows.Next() {
		err := rows.Scan(&token)
		if err != nil {
			log.Error(err)
			continue
		}
		out = append(out, token)
	}
	return out, nil
}
