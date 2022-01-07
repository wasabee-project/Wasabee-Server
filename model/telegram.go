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

// Delete is used to remove a TelegramID
func (tgid TelegramID) Delete() error {
	if _, err := db.Exec("DELETE FROM telegram WHERE telegramID = ?", tgid); err != nil {
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

// RemoveTelegramID clears any telegram configuration for a given agent
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

// UnverifyAgent marks an ID as unverified (if the agent blocks the bot)
func (tgid TelegramID) UnverifyAgent() error {
	_, err := db.Exec("UPDATE telegram SET verified = 0 WHERE telegramID = ?", tgid)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// String returns a string format of a TelegramID
func (tgid TelegramID) String() string {
	return strconv.Itoa(int(tgid))
}

// LinkToTelegramChat associates a telegram chat ID with the team, performs authorization
// a chat can be linked to a single team
// a team can be linked to a single chat (irrespective of opID)
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
func (teamID TeamID) UnlinkFromTelegramChat() error {
	_, err := db.Exec("DELETE FROM telegramteam WHERE teamID = ?", teamID)
	if err != nil {
		log.Error(err)
		return err
	}

	log.Debugw("unlinked team from telegram", "teamID", teamID)
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

// AddToChatMemberList notes a telegramID has been seen in a given telegram chat
func AddToChatMemberList(agent TelegramID, chat TelegramID) error {
	if _, err := db.Exec("REPLACE INTO telegramchatmembers (agent, chat) VALUES (?, ?)", agent, chat); err != nil {
		// log.Debug(err) // foreign key errors due to chat not being linked can be ignored
		return err
	}
	return nil
}

// IsChatMember reports if a given telegramID has been seen in a given telegram chat
func IsChatMember(agent TelegramID, chat TelegramID) bool {
	var i bool

	err := db.QueryRow("SELECT COUNT(*) FROM telegramchatmembers WHERE agent = ? AND chat = ?", agent, chat).Scan(&i)
	if err != nil {
		return false
	}
	return i
}

// RemoveFromChatMemberList removes the agent from the list for the telegram chat
func RemoveFromChatMemberList(agent TelegramID, chat TelegramID) error {
	if _, err := db.Exec("DELETE FROM telegramchatmembers WHERE agent=? AND chat=?", agent, chat); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// GetAllTelegramIDs is used by the telegram cleanup function
func GetAllTelegramIDs() ([]TelegramID, error) {
	var tgs []TelegramID

	rows, err := db.Query("SELECT telegramID FROM telegram")
	if err != nil {
		log.Error(err)
		return tgs, err
	}
	defer rows.Close()

	for rows.Next() {
		var tg TelegramID
		if err := rows.Scan(&tg); err != nil {
			log.Error(err)
			continue
		}
		tgs = append(tgs, tg)
	}

	return tgs, nil
}
