package model

import (
	"database/sql"
	"fmt"

	"github.com/wasabee-project/Wasabee-Server/log"
)

// OneTimeToken - used to authenticate users in IITC when GAPI doesn't work for them
type OneTimeToken string

// String is a stringer for OTT
func (ott OneTimeToken) String() string {
	return string(ott)
}

// Gid converts a location share key to a agent's gid
func (ott OneTimeToken) Gid() (GoogleID, error) {
	var gid GoogleID

	err := db.QueryRow("SELECT gid FROM agent WHERE OneTimeToken = ?", ott).Scan(&gid)
	if err != nil && err == sql.ErrNoRows {
		err := fmt.Errorf(ErrInvalidOTT)
		log.Info(err)
		return "", err
	}
	if err != nil {
		log.Info(err)
		return "", err
	}

	return gid, nil
}

// NewOneTimeToken generates a new OTT for an agent
func (gid GoogleID) newOneTimeToken() (OneTimeToken, error) {
	ott, err := GenerateSafeName()
	if err != nil {
		log.Error(err)
		return "", err
	}
	if _, err = db.Exec("UPDATE agent SET OneTimeToken = ? WHERE gid = ?", ott, gid); err != nil {
		log.Error(err)
		return "", err
	}
	return OneTimeToken(ott), nil
}

// Increment "uses" the OTT and returns a googleID, replacing the agent's OTT in the databse
func (ott OneTimeToken) Increment() (GoogleID, error) {
	gid, err := ott.Gid()
	if err != nil {
		log.Error(err)
		return "", err
	}

	_, err = gid.newOneTimeToken()
	if err != nil {
		log.Warn(err)
	}
	return gid, nil
}
