package model

import (
	"context"
	"database/sql"

	"github.com/wasabee-project/Wasabee-Server/log"
)

// IntelFaction is stored as an int8 in the database
type IntelFaction int8

const (
	FactionUnset IntelFaction = -1
	FactionRes   IntelFaction = 0
	FactionEnl   IntelFaction = 1
)

// FactionFromString takes a string and returns the corresponding IntelFaction
func FactionFromString(in string) IntelFaction {
	switch in {
	case "RESISTANCE", "RES", "res", "0":
		return FactionRes
	case "ENLIGHTENED", "ENL", "enl", "1":
		return FactionEnl
	default:
		return FactionUnset
	}
}

// String returns the string representation of an IntelFaction
func (f IntelFaction) String() string {
	switch f {
	case FactionRes:
		return "RESISTANCE"
	case FactionEnl:
		return "ENLIGHTENED"
	default:
		return "unset"
	}
}

// SetFaction updates the agent's faction in the database
func (gid GoogleID) SetFaction(ctx context.Context, faction IntelFaction) error {
	_, err := db.ExecContext(ctx, "UPDATE agent SET faction = ? WHERE gid = ?", faction, gid)
	if err != nil {
		log.Error(err)
	}
	return err
}

// GetFaction returns the agent's stored faction
func (gid GoogleID) GetFaction(ctx context.Context) (IntelFaction, error) {
	var f IntelFaction
	err := db.QueryRowContext(ctx, "SELECT faction FROM agent WHERE gid = ?", gid).Scan(&f)
	if err != nil {
		if err == sql.ErrNoRows {
			return FactionUnset, nil
		}
		return FactionUnset, err
	}
	return f, nil
}
