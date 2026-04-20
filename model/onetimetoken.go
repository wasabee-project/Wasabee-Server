package model

import (
	"context"
	"database/sql"
	"errors"

	"github.com/wasabee-project/Wasabee-Server/log"
)

// OneTimeToken - used to authenticate users in IITC when GAPI doesn't work for them
type OneTimeToken string

// String is a stringer for OTT
func (ott OneTimeToken) String() string {
	return string(ott)
}

// Gid converts an OTT to an agent's gid
func (ott OneTimeToken) Gid(ctx context.Context) (GoogleID, error) {
	var gid GoogleID

	err := db.QueryRowContext(ctx, "SELECT gid FROM agent WHERE OneTimeToken = ?", ott).Scan(&gid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New(ErrInvalidOTT)
		}
		log.Error(err)
		return "", err
	}

	return gid, nil
}

// newOneTimeToken generates a new OTT for an agent
func (gid GoogleID) newOneTimeToken(ctx context.Context) (OneTimeToken, error) {
	ott, err := GenerateSafeName(ctx)
	if err != nil {
		log.Error(err)
		return "", err
	}

	if _, err = db.ExecContext(ctx, "UPDATE agent SET OneTimeToken = ? WHERE gid = ?", ott, gid); err != nil {
		log.Error(err)
		return "", err
	}
	return OneTimeToken(ott), nil
}

// Increment "uses" the OTT and returns a googleID, replacing the agent's OTT in the database
func (ott OneTimeToken) Increment(ctx context.Context) (GoogleID, error) {
	gid, err := ott.Gid(ctx)
	if err != nil {
		return "", err
	}

	// Generate a new one to replace the one just used
	_, err = gid.newOneTimeToken(ctx)
	if err != nil {
		log.Warnw("failed to refresh OTT after use", "gid", gid, "error", err)
	}
	return gid, nil
}
