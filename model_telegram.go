package PhDevBin

import (
	// "database/sql"
	"errors"
	"fmt"
	// "strconv"
	"github.com/go-telegram-bot-api/telegram-bot-api"
)

var tgbot *tgbotapi.User
var tgrunning bool

// TGSetBot is called from the Telegram bot startup to let other services know it is running
func TGSetBot(b *tgbotapi.User) {
	tgbot = b
	tgrunning = true
}

// TGGetBotName returns the bot's telegram username
// used by templates
func TGGetBotName() (string, error) {
	if tgrunning == false {
		return "", nil
	}
	return tgbot.UserName, nil
}

// TGGetBotID returns the bot's telegram ID number
// used by templates
func TGGetBotID() (int, error) {
	if tgrunning == false {
		return 0, nil
	}
	return tgbot.ID, nil
}

// TGRunning is used by templates to determine if they should display telegram info
func TGRunning() (bool, error) {
	return tgrunning, nil
}

// TelegramToGid returns a gid and V verified status for a given Telegram ID #
// TODO: some places the tgid is int and others int64 - sort this out
func TelegramToGid(tgid int) (string, bool, error) {
	var gid string
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

// GidToTelegram returns a telegram ID number for a gid
func GidToTelegram(gid string) (int64, error) {
	var tgid int64

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
func TelegramInitUser(ID int, name string, lockey string) error {
	authtoken := GenerateName()

	gid, err := LockeyToGid(lockey)
	if err != nil && err.Error() == "sql: no rows in result set" {
		e := fmt.Sprintf("Location Share Key (%s) is not recognized", lockey)
		return errors.New(e)
	}

	if err != nil {
		Log.Notice(err)
		return err
	}
	_, err = db.Exec("INSERT INTO telegram VALUES (?, ?, ?, 0, ?)", ID, name, gid, authtoken)
	if err != nil {
		Log.Notice(err)
		return err
	}

	return nil
}

// TelegramInitUser2 is the second stage of the verication process
func TelegramInitUser2(ID int, authtoken string) error {
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
