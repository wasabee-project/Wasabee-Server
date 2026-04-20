package model

import (
	"context"
	"database/sql"
	"time"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/util"
)

// RocksAgent is defined by enlightened.rocks
type RocksAgent struct {
	Gid      GoogleID `json:"gid"`
	Agent    string   `json:"agentid"`
	TGId     int64    `json:"tgid"`
	Verified bool     `json:"verified"`
	Smurf    bool     `json:"smurf"`
}

// RocksToDB writes a rocks agent to the database
func RocksToDB(ctx context.Context, a *RocksAgent) error {
	if a.Agent == "" || a.Gid == "" {
		return nil
	}

	if len(a.Agent) > 15 {
		log.Infow("long agent name from Rocks", "gid", a.Gid, "name", a.Agent)
	}

	// REPLACE is used because this is essentially a cache of the Rocks data
	_, err := db.ExecContext(ctx, "REPLACE INTO rocks (gid, tgid, agent, verified, smurf, fetched) VALUES (?, ?, LEFT(?, 15), ?, ?, UTC_TIMESTAMP())",
		a.Gid, a.TGId, a.Agent, a.Verified, a.Smurf)
	if err != nil {
		log.Error(err)
		return err
	}

	// Trust Rocks verification for Telegram info
	if a.TGId > 0 {
		existing, err := a.Gid.TelegramID(ctx)
		if err != nil {
			log.Error(err)
			return err
		}
		if existing == 0 {
			if _, err := db.ExecContext(ctx, "INSERT IGNORE INTO telegram (telegramID, gid, verified) VALUES (?, ?, 1)", a.TGId, a.Gid); err != nil {
				log.Error(err)
				return err
			}
		}
	}
	return nil
}

// RocksFromDB returns a rocks agent from the database cache
func RocksFromDB(ctx context.Context, gid GoogleID) (*RocksAgent, time.Time, error) {
	var a RocksAgent
	var fetched time.Time
	var tgid sql.NullInt64
	var agent sql.NullString

	// Scanning directly into fetched (time.Time) works if the DSN has parseTime=true
	err := db.QueryRowContext(ctx, "SELECT gid, tgid, agent, verified, smurf, fetched FROM rocks WHERE gid = ?", gid).
		Scan(&a.Gid, &tgid, &agent, &a.Verified, &a.Smurf, &fetched)

	if err != nil {
		if err == sql.ErrNoRows {
			return &a, time.Time{}, nil
		}
		log.Error(err)
		return nil, time.Time{}, err
	}

	if agent.Valid {
		a.Agent = agent.String
	}
	if tgid.Valid {
		a.TGId = tgid.Int64
	}

	return &a, fetched, nil
}

// RocksCommunity returns a communityID for a TeamID
func (teamID TeamID) RocksCommunity(ctx context.Context) (string, error) {
	var rc sql.NullString
	err := db.QueryRowContext(ctx, "SELECT rockscomm FROM team WHERE teamID = ?", teamID).Scan(&rc)
	if err != nil {
		return "", err
	}
	return rc.String, nil
}

// RocksKey returns a rocks key for a TeamID
func (teamID TeamID) RocksKey(ctx context.Context) (string, error) {
	var rc sql.NullString
	err := db.QueryRowContext(ctx, "SELECT rockskey FROM team WHERE teamID = ?", teamID).Scan(&rc)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return rc.String, nil
}

// RocksCommunityToTeam returns a TeamID from a Rocks Community ID
func RocksCommunityToTeam(ctx context.Context, communityID string) (TeamID, error) {
	var teamID TeamID
	err := db.QueryRowContext(ctx, "SELECT teamID FROM team WHERE rockscomm = ?", communityID).Scan(&teamID)
	if err != nil {
		return "", err
	}
	return teamID, nil
}

// SetRocks links a team to a community at enl.rocks.
func (teamID TeamID) SetRocks(ctx context.Context, key, community string) error {
	k := makeNullString(util.Sanitize(key))
	c := makeNullString(util.Sanitize(community))

	_, err := db.ExecContext(ctx, "UPDATE team SET rockskey = ?, rockscomm = ? WHERE teamID = ?", k, c, teamID)
	return err
}
