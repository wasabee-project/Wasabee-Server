package model

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/wasabee-project/Wasabee-Server/generatename"
	"github.com/wasabee-project/Wasabee-Server/log"
)

// TelegramID is a user ID from telegram
type TelegramID int64

// GidV returns a googleID/verified pair for a given telegram ID
func (tgid TelegramID) GidV() (GoogleID, bool, error) {
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
	gid, v, err := tgid.GidV()
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
		err = fmt.Errorf("token not recognized")
		log.Debugw(err.Error(), "resource", ott, "tgid", tgid, "name", name)
		return err
	}
	if err != nil {
		log.Error(err)
		return err
	}

	if _, err = db.Exec("INSERT INTO telegram (telegramID, telegramName, gid, verified, authtoken) VALUES (?, ?, ?, 0, ?)", tgid, name, gid, authtoken); err != nil {
		log.Info(err)
		return err
	}

	return nil
}

// SetName is used to set an agent's telegram display name
func (tgid TelegramID) SetName(name string) error {
	if _, err := db.Exec("UPDATE telegram SET telegramName = ? WHERE telegramID = ?", name, tgid); err != nil {
		log.Info(err)
		return err
	}
	return nil
}

// SetTelegramID adds a verified agent's telegram ID
func (gid GoogleID) SetTelegramID(tgid TelegramID, name string) error {
	if _, err := db.Exec("INSERT INTO telegram (gid, telegramID, telegramName, verified) VALUES (?, ?, ?, 1)", gid, tgid, name); err != nil {
		log.Info(err)
		return err
	}
	return nil
}

func (gid GoogleID) RemoveTelegramID() error {
	if _, err := db.Exec("DELETE FROM telegram WHERE gid = ?", gid); err != nil {
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

// LinkToTelegramChat associates a telegram chat ID with the team, performs authorization
func (teamID TeamID) LinkToTelegramChat(chat TelegramID, opID OperationID) error {
	log.Debugw("linking team to chat", "chat", chat, "teamID", teamID, "opID", opID)

	_, err := db.Exec("REPLACE INTO telegramteam (teamID, telegram, opID) VALUES (?,?,?)", teamID, chat, MakeNullString(string(opID)))
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// UnlinkFromTelegramChat disassociates a telegram chat ID from the team -- not authenticated since bot removal from chat is enough
func (teamID TeamID) UnlinkFromTelegramChat(chat TelegramID) error {
	_, err := db.Exec("DELETE FROM telegramteam WHERE teamID = ? AND telegram = ?", teamID, chat)
	if err != nil {
		log.Error(err)
		return err
	}

	log.Debugw("unlinked team from telegram", "teamID", teamID, "chat", chat)
	return nil
}

// TelegramChat returns the associated telegram chat ID for this team, if any
func (teamID TeamID) TelegramChat() (int64, error) {
	var chatID int64

	err := db.QueryRow("SELECT telegram FROM telegramteam WHERE teamID = ?", teamID).Scan(&chatID)
	if err != nil && err != sql.ErrNoRows {
		log.Error(err)
		return chatID, err
	}
	if err == sql.ErrNoRows {
		return chatID, nil
	}
	return chatID, nil
}

// ChatToTeam takes a chatID and returns a linked teamID
func ChatToTeam(chat int64) (TeamID, OperationID, error) {
	var t TeamID
	var o OperationID
	var on sql.NullString

	err := db.QueryRow("SELECT teamID, opID FROM telegramteam WHERE telegram = ?", chat).Scan(&t, &on)
	if err != nil && err != sql.ErrNoRows {
		log.Error(err)
		return t, o, err
	}
	if err == sql.ErrNoRows {
		err := fmt.Errorf("chat not linked to any teams")
		return t, o, err
	}
	if on.Valid {
		o = OperationID(on.String)
	}
	return t, o, nil
}
