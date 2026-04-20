package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/util"
)

// TelegramID is a user ID from telegram
type TelegramID int64

// GidV returns a googleID/verified pair for a given telegram ID
func (tgid TelegramID) GidV(ctx context.Context) (GoogleID, bool, error) {
	var gid GoogleID
	var verified bool

	err := db.QueryRowContext(ctx, "SELECT gid, verified FROM telegram WHERE telegramID = ?", tgid).Scan(&gid, &verified)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		log.Error(err)
		return "", false, err
	}
	return gid, verified, nil
}

// Gid returns a verified GoogleID for a Telegram ID
func (tgid TelegramID) Gid(ctx context.Context) (GoogleID, error) {
	gid, v, err := tgid.GidV(ctx)
	if err != nil {
		return "", err
	}
	if !v {
		return "", fmt.Errorf("unverified")
	}
	return gid, nil
}

// TelegramID returns a telegram ID number for a gid
func (gid GoogleID) TelegramID(ctx context.Context) (TelegramID, error) {
	var tgid TelegramID

	err := db.QueryRowContext(ctx, "SELECT telegramID FROM telegram WHERE gid = ?", gid).Scan(&tgid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		log.Error(err)
		return 0, err
	}
	return tgid, nil
}

// TelegramName returns a telegram friendly name for a gid
func (gid GoogleID) TelegramName(ctx context.Context) (string, error) {
	var tgName sql.NullString

	err := db.QueryRowContext(ctx, "SELECT telegramName FROM telegram WHERE gid = ?", gid).Scan(&tgName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		log.Error(err)
		return "", err
	}
	return tgName.String, nil
}

// InitAgent establishes a new telegram user in the database and begins the verification process
func (tgid TelegramID) InitAgent(ctx context.Context, name string, ott OneTimeToken) error {
	authtoken := util.GenerateName()
	gid, err := ott.Gid(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Debugw("token not recognized", "resource", ott, "tgid", tgid, "name", name)
			return fmt.Errorf("token not recognized")
		}
		log.Error(err)
		return err
	}

	_, err = db.ExecContext(ctx, "INSERT INTO telegram (telegramID, telegramName, gid, verified, authtoken) VALUES (?, ?, ?, 0, ?)",
		tgid, name, gid, authtoken)
	return err
}

// SetName is used to set an agent's telegram display name
func (tgid TelegramID) SetName(ctx context.Context, name string) error {
	_, err := db.ExecContext(ctx, "UPDATE telegram SET telegramName = ? WHERE telegramID = ?", name, tgid)
	return err
}

// Delete is used to remove a TelegramID
func (tgid TelegramID) Delete(ctx context.Context) error {
	_, err := db.ExecContext(ctx, "DELETE FROM telegram WHERE telegramID = ?", tgid)
	return err
}

// SetTelegramID adds a verified agent's telegram ID directly
func (gid GoogleID) SetTelegramID(ctx context.Context, tgid TelegramID, name string) error {
	_, err := db.ExecContext(ctx, "INSERT INTO telegram (gid, telegramID, telegramName, verified) VALUES (?, ?, ?, 1)",
		gid, tgid, name)
	return err
}

// RemoveTelegramID clears any telegram configuration for a given agent
func (gid GoogleID) RemoveTelegramID(ctx context.Context) error {
	_, err := db.ExecContext(ctx, "DELETE FROM telegram WHERE gid = ?", gid)
	return err
}

// VerifyAgent is the second stage of the verification process
func (tgid TelegramID) VerifyAgent(ctx context.Context, authtoken string) error {
	res, err := db.ExecContext(ctx, "UPDATE telegram SET authtoken = NULL, verified = 1 WHERE telegramID = ? AND authtoken = ?",
		tgid, authtoken)
	if err != nil {
		return err
	}

	i, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if i < 1 {
		return fmt.Errorf("invalid AuthToken")
	}

	return nil
}

// UnverifyAgent marks an ID as unverified (if the agent blocks the bot)
func (tgid TelegramID) UnverifyAgent(ctx context.Context) error {
	_, err := db.ExecContext(ctx, "UPDATE telegram SET verified = 0 WHERE telegramID = ?", tgid)
	return err
}

func (tgid TelegramID) String() string {
	return strconv.FormatInt(int64(tgid), 10)
}

// LinkToTelegramChat associates a telegram chat ID with the team
func (teamID TeamID) LinkToTelegramChat(ctx context.Context, chat TelegramID, opID OperationID) error {
	log.Debugw("linking team to chat", "chat", chat, "teamID", teamID, "opID", opID)

	_, err := db.ExecContext(ctx, "REPLACE INTO telegramteam (teamID, telegram, opID) VALUES (?, ?, ?)",
		teamID, chat, makeNullString(string(opID)))
	return err
}

// UnlinkFromTelegramChat disassociates a telegram chat ID from the team
func (teamID TeamID) UnlinkFromTelegramChat(ctx context.Context) error {
	_, err := db.ExecContext(ctx, "DELETE FROM telegramteam WHERE teamID = ?", teamID)
	return err
}

// TelegramChat returns the associated telegram chat ID for this team
func (teamID TeamID) TelegramChat(ctx context.Context) (int64, error) {
	var chatID int64
	err := db.QueryRowContext(ctx, "SELECT telegram FROM telegramteam WHERE teamID = ?", teamID).Scan(&chatID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	return chatID, nil
}

// ChatToTeam takes a chatID and returns a linked teamID and OpID
func ChatToTeam(ctx context.Context, chat int64) (TeamID, OperationID, error) {
	var t TeamID
	var on sql.NullString

	err := db.QueryRowContext(ctx, "SELECT teamID, opID FROM telegramteam WHERE telegram = ?", chat).Scan(&t, &on)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", fmt.Errorf("chat not linked to any teams")
		}
		return "", "", err
	}

	var o OperationID
	if on.Valid {
		o = OperationID(on.String)
	}
	return t, o, nil
}

// AddToChatMemberList notes a telegramID has been seen in a given telegram chat
func AddToChatMemberList(ctx context.Context, agent TelegramID, chat TelegramID) error {
	_, err := db.ExecContext(ctx, "INSERT IGNORE INTO telegramchatmembers (agent, chat) VALUES (?, ?)", agent, chat)
	return err
}

// IsChatMember reports if a given telegramID has been seen in a given telegram chat
func IsChatMember(ctx context.Context, agent TelegramID, chat TelegramID) bool {
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM telegramchatmembers WHERE agent = ? AND chat = ?", agent, chat).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

// RemoveFromChatMemberList removes the agent from the list for the telegram chat
func RemoveFromChatMemberList(ctx context.Context, agent TelegramID, chat TelegramID) error {
	_, err := db.ExecContext(ctx, "DELETE FROM telegramchatmembers WHERE agent=? AND chat=?", agent, chat)
	return err
}

// GetAllTelegramIDs is used by the telegram cleanup function
func GetAllTelegramIDs(ctx context.Context) ([]TelegramID, error) {
	rows, err := db.QueryContext(ctx, "SELECT telegramID FROM telegram")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tgs []TelegramID
	for rows.Next() {
		var tg TelegramID
		if err := rows.Scan(&tg); err != nil {
			continue
		}
		tgs = append(tgs, tg)
	}
	return tgs, nil
}
