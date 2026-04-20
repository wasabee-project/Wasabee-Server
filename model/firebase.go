package model

import (
	"context"
	"database/sql"
	"errors"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
)

// GetFirebaseTokens gets an agent's FirebaseTokens from the database
func (gid GoogleID) GetFirebaseTokens(ctx context.Context) ([]string, error) {
	rows, err := db.QueryContext(ctx, "SELECT DISTINCT token FROM firebase WHERE gid = ?", gid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		log.Error(err)
		return nil, err
	}
	defer rows.Close()

	var toks []string
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			continue
		}
		toks = append(toks, token)
	}

	return toks, nil
}

// StoreFirebaseToken adds a token in the database for an agent.
// An agent may have any number of tokens (e.g. multiple devices/browsers).
func (gid GoogleID) StoreFirebaseToken(ctx context.Context, token string) error {
	// Use INSERT IGNORE to skip if the gid/token pair already exists, reducing DB round-trips
	_, err := db.ExecContext(ctx, "INSERT IGNORE INTO firebase (gid, token) VALUES (?, ?)", gid, token)
	if err != nil {
		log.Error(err)
		return err
	}

	// Subscribe this token to all team topics
	// Note: messaging.AddToRemote likely handles the heavy lifting of talking to FCM/Google
	teams := gid.TeamList(ctx)

	for _, teamID := range teams {
		messaging.AddToRemote(ctx, messaging.GoogleID(gid), messaging.TeamID(teamID))
	}

	return nil
}

// RemoveFirebaseToken removes a given token from the database
func RemoveFirebaseToken(ctx context.Context, token string) error {
	_, err := db.ExecContext(ctx, "DELETE FROM firebase WHERE token = ?", token)
	return err
}

// RemoveAllFirebaseTokens removes all tokens for a given agent
func (gid GoogleID) RemoveAllFirebaseTokens(ctx context.Context) error {
	_, err := db.ExecContext(ctx, "DELETE FROM firebase WHERE gid = ?", gid)
	return err
}

// FirebaseBroadcastList returns all known firebase tokens for messaging all agents
func FirebaseBroadcastList(ctx context.Context) ([]string, error) {
	rows, err := db.QueryContext(ctx, "SELECT DISTINCT token FROM firebase")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
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

// FirebaseLocationTokens returns all tokens for agents on teams where this agent is sharing location
func (gid GoogleID) FirebaseLocationTokens(ctx context.Context) ([]TeamToken, error) {
	// This query finds all tokens belonging to agents who share a team with 'gid'
	// where 'gid' has opted to share their location.
	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT f.token, at.teamID 
		FROM firebase f
		JOIN agentteams at ON f.gid = at.gid 
		WHERE at.teamID IN (SELECT teamID FROM agentteams WHERE gid = ? AND shareLoc = 1)`, gid)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TeamToken
	for rows.Next() {
		var tt TeamToken
		if err := rows.Scan(&tt.Token, &tt.TeamID); err != nil {
			continue
		}
		out = append(out, tt)
	}
	return out, nil
}
