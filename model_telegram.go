package PhDevBin

import (
	"errors"
	"fmt"
	// "github.com/go-telegram-bot-api/telegram-bot-api"
)

var tgbotname string
var tgbotid int
var tgrunning bool

// TelegramID is a user ID from telegram -- used for V
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
	if tgrunning == false {
		return "", nil
	}
	return tgbotname, nil
}

// TGGetBotID returns the bot's telegram ID number
// used by templates
func TGGetBotID() (int, error) {
	if tgrunning == false {
		return 0, nil
	}
	return tgbotid, nil
}

// TGRunning is used by templates to determine if they should display telegram info
func TGRunning() (bool, error) {
	return tgrunning, nil
}

// GidV returns a gid and V verified status for a given Telegram ID #
func (tgid TelegramID) GidV() (GoogleID, bool, error) {
	var gid GoogleID
	var verified bool

	row := db.QueryRow("SELECT gid, verified FROM telegram WHERE telegramID = ?", tgid)
	err := row.Scan(&gid, &verified)
	if err != nil && err.Error() == "sql: no rows in result set" {
		return "", false, nil
	}
	if err != nil {
		Log.Notice(err)
		return "", false, err
	}
	return gid, verified, nil
}

// TelegramID returns a telegram ID number for a gid
func (gid GoogleID) TelegramID() (int, error) {
	var tgid int

	row := db.QueryRow("SELECT telegramID FROM telegram WHERE gid = ?", gid)
	err := row.Scan(&tgid)
	if err != nil && err.Error() == "sql: no rows in result set" {
		return 0, nil
	}
	if err != nil {
		Log.Notice(err)
		return 0, err
	}
	return tgid, nil
}

// TelegramInitUser establishes a new telegram user in the database and begins the verification process
func TelegramInitUser(ID int, name string, lockey LocKey) error {
	authtoken := GenerateName()

	gid, err := lockey.Gid()
	if err != nil && err.Error() == "sql: no rows in result set" {
		e := fmt.Sprintf("Location Share Key (%s) is not recognized", lockey)
		return errors.New(e)
	}

	if err != nil {
		Log.Notice(err)
		return err
	}
	_, err = db.Exec("INSERT INTO telegram (telegramID, telegramName, gid, verified, authtoken) VALUES (?, ?, ?, 0, ?)", ID, name, gid, authtoken)
	if err != nil {
		Log.Notice(err)
		return err
	}

	return nil
}

// TelegramVerifyUser is the second stage of the verication process
func TelegramVerifyUser(ID int, authtoken string) error {
	res, err := db.Exec("UPDATE telegram SET authtoken = NULL, verified = 1 WHERE telegramID = ? AND authtoken = ?", ID, authtoken)
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
		return errors.New("Invalid AuthToken")
	} // trust the primary key prevents i > 1

	return nil
}
