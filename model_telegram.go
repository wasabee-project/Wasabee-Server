package wasabi

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
)

var tgbotname string
var tgbotid int
var tgrunning bool

// TelegramID is a user ID from telegram
type TelegramID int

// TGSetBot is called from the Telegram bot startup to let other services know it is running
func TGSetBot(botname string, botid int) {
	tgbotname = botname
	tgbotid = botid
	tgrunning = true
}

// TGGetBotName returns the bot's telegram username
// used by templates
func TGGetBotName() (string, error) {
	if !tgrunning {
		return "", nil
	}
	return tgbotname, nil
}

// TGGetBotID returns the bot's telegram ID number
// used by templates
func TGGetBotID() (int, error) {
	if !tgrunning {
		return 0, nil
	}
	return tgbotid, nil
}

// TGRunning is used by templates to determine if they should display telegram info
func TGRunning() (bool, error) {
	return tgrunning, nil
}

// GidV returns a GoogleID and V verified status for a given Telegram ID #
func (tgid TelegramID) GidV() (GoogleID, bool, error) {
	var gid GoogleID
	var verified bool

	row := db.QueryRow("SELECT gid, verified FROM telegram WHERE telegramID = ?", tgid)
	err := row.Scan(&gid, &verified)
	if err != nil && err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		Log.Notice(err)
		return "", false, err
	}
	return gid, verified, nil
}

// Gid returns a verified GoogleID for a Telegram ID
func (tgid TelegramID) Gid() (GoogleID, error) {
	gid, v, err := tgid.GidV()
	if err != nil {
		return "", err
	}
	if !v {
		return "", fmt.Errorf("Unverified")
	}
	return gid, nil
}

// TelegramID returns a telegram ID number for a gid
func (gid GoogleID) TelegramID() (TelegramID, error) {
	var tgid TelegramID

	row := db.QueryRow("SELECT telegramID FROM telegram WHERE gid = ?", gid)
	err := row.Scan(&tgid)
	if err != nil && err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		Log.Notice(err)
		return 0, err
	}
	return tgid, nil
}

// InitAgent establishes a new telegram user in the database and begins the verification process
func (tgid TelegramID) InitAgent(name string, lockey LocKey) error {
	authtoken := GenerateName()

	gid, err := lockey.Gid()
	if err != nil && err == sql.ErrNoRows {
		e := fmt.Sprintf("Location Share Key (%s) is not recognized", lockey)
		return errors.New(e)
	}

	if err != nil {
		Log.Notice(err)
		return err
	}
	_, err = db.Exec("INSERT INTO telegram (telegramID, telegramName, gid, verified, authtoken) VALUES (?, ?, ?, 0, ?)", tgid, name, gid, authtoken)
	if err != nil {
		Log.Notice(err)
		return err
	}

	return nil
}

// VerifyAgent is the second stage of the verication process
func (tgid TelegramID) VerifyAgent(authtoken string) error {
	res, err := db.Exec("UPDATE telegram SET authtoken = NULL, verified = 1 WHERE telegramID = ? AND authtoken = ?", tgid, authtoken)
	if err != nil {
		Log.Notice(err)
		return err
	}
	i, err := res.RowsAffected()
	if err != nil {
		Log.Notice(err)
		return err
	}

	if i < 1 {
		return errors.New("invalid AuthToken")
	} // trust the primary key prevents i > 1

	return nil
}

// String returns a string format of a TelegramID
func (tgid TelegramID) String() string {
	return strconv.Itoa(int(tgid))
}
