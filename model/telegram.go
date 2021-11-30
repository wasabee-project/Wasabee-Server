package model

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/wasabee-project/Wasabee-Server/generatename"
	"github.com/wasabee-project/Wasabee-Server/log"
)

// TelegramID is a user ID from telegram
type TelegramID int

func (tgid TelegramID) gidV() (GoogleID, bool, error) {
	var gid GoogleID
	var verified bool

	err := db.QueryRow("SELECT gid, verified FROM telegram WHERE telegramID = ?", tgid).Scan(&gid, &verified)
	if err != nil && err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		log.Error(err)
		return "", false, err
	}
	return gid, verified, nil
}

// Gid returns a verified GoogleID for a Telegram ID
func (tgid TelegramID) Gid() (GoogleID, error) {
	gid, v, err := tgid.gidV()
	if err != nil {
		return "", err
	}
	if !v {
		return "", fmt.Errorf("unverified")
	}
	return gid, nil
}

// TelegramID returns a telegram ID number for a gid
func (gid GoogleID) TelegramID() (TelegramID, error) {
	var tgid TelegramID

	err := db.QueryRow("SELECT telegramID FROM telegram WHERE gid = ?", gid).Scan(&tgid)
	if err != nil && err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		log.Error(err)
		return 0, err
	}
	return tgid, nil
}

// TelegramName returns a telegram friendly name for a gid
func (gid GoogleID) TelegramName() (string, error) {
	var tgName sql.NullString

	err := db.QueryRow("SELECT telegramName FROM telegram WHERE gid = ?", gid).Scan(&tgName)
	if (err != nil && err == sql.ErrNoRows) || !tgName.Valid {
		return "", nil
	}
	if err != nil {
		log.Error(err)
		return "", err
	}
	return tgName.String, nil
}

// InitAgent establishes a new telegram user in the database and begins the verification process
func (tgid TelegramID) InitAgent(name string, ott OneTimeToken) error {
	authtoken := generatename.GenerateName()

	gid, err := ott.Gid()
	if err != nil && err == sql.ErrNoRows {
		err = fmt.Errorf("location share key is not recognized")
		log.Warnw(err.Error(), "resource", ott, "tgid", tgid, "name", name)
		return err
	}
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = db.Exec("INSERT INTO telegram (telegramID, telegramName, gid, verified, authtoken) VALUES (?, ?, ?, 0, ?)", tgid, name, gid, authtoken)
	if err != nil {
		log.Info(err)
		return err
	}

	return nil
}

// UpdateName is used to set an agent's telegram display name
func (tgid TelegramID) UpdateName(name string) error {
	_, err := db.Exec("UPDATE telegram SET telegramName = ? WHERE telegramID = ?", name, tgid)
	if err != nil {
		log.Info(err)
		return err
	}
	return nil
}

// VerifyAgent is the second stage of the verication process
func (tgid TelegramID) VerifyAgent(authtoken string) error {
	res, err := db.Exec("UPDATE telegram SET authtoken = NULL, verified = 1 WHERE telegramID = ? AND authtoken = ?", tgid, authtoken)
	if err != nil {
		log.Error(err)
		return err
	}
	i, err := res.RowsAffected()
	if err != nil {
		log.Error(err)
		return err
	}

	if i < 1 {
		err = fmt.Errorf("invalid AuthToken")
		log.Warnw(err.Error(), "tgid", tgid)
		return err
	} // trust the primary key prevents i > 1

	return nil
}

// String returns a string format of a TelegramID
func (tgid TelegramID) String() string {
	return strconv.Itoa(int(tgid))
}
