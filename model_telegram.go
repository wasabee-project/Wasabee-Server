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

func TGSetBot(b *tgbotapi.User) {
	tgbot = b
	tgrunning = true
}

func TGGetBotName() (string, error) {
	if tgrunning == false {
		return "", nil
	}
	return tgbot.UserName, nil
}

func TGGetBotID() (int, error) {
	if tgrunning == false {
		return 0, nil
	}
	return tgbot.ID, nil
}

func TGRunning() (bool, error) {
	return tgrunning, nil
}

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
