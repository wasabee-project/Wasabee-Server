package model

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/util"
)

// KeyOnHand describes the keys already in possession for the op
type KeyOnHand struct {
	ID      PortalID `json:"portalId"`
	Gid     GoogleID `json:"gid"`
	Capsule string   `json:"capsule"`
	Onhand  int32    `json:"onhand"`
}

// insertKey adds a user keycount to the database
func (o *Operation) insertKey(ctx context.Context, k KeyOnHand, tx *sql.Tx) error {
	// Use the transaction-safe internal portal details check
	details, err := o.portalDetails(ctx, k.ID, tx)
	if err != nil {
		log.Error(err)
		return err
	}

	// If the portal isn't in the op, we shouldn't have key counts for it
	if details == nil || details.Name == "" {
		log.Infow("attempt to assign key count to portal not in op", "GID", k.Gid, "resource", o.ID, "portal", k.ID)
		_, err = tx.ExecContext(ctx, "DELETE FROM opkeys WHERE opID = ? AND portalID = ?", o.ID, k.ID)
		if err != nil {
			return errors.New(ErrKeyUnableToRemove)
		}
		return nil
	}

	k.Capsule = util.Sanitize(k.Capsule)

	executor := txExecutor(tx)
	if k.Onhand == 0 {
		_, err = executor.ExecContext(ctx, "DELETE FROM opkeys WHERE opID = ? AND portalID = ? AND gid = ? AND capsule = ?", o.ID, k.ID, k.Gid, k.Capsule)
		if err != nil {
			return errors.New(ErrKeyUnableToRemove)
		}
	} else {
		// REPLACE is appropriate here as (opID, portalID, gid, capsule) is the unique key
		_, err = executor.ExecContext(ctx, "REPLACE INTO opkeys (opID, portalID, gid, onhand, capsule) VALUES (?, ?, ?, ?, ?)", o.ID, k.ID, k.Gid, k.Onhand, k.Capsule)
		if err != nil {
			if strings.Contains(err.Error(), "Error 1452") { // Foreign key constraint
				return errors.New(ErrKeyUnableToRecord)
			}
			return err
		}
	}
	return nil
}

// populateKeys fills in the Keys on hand list for the Operation.
func (o *Operation) populateKeys(ctx context.Context) error {
	rows, err := db.QueryContext(ctx, "SELECT portalID, gid, onhand, capsule FROM opkeys WHERE opID = ?", o.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var k KeyOnHand
		var cap sql.NullString
		if err := rows.Scan(&k.ID, &k.Gid, &k.Onhand, &cap); err != nil {
			continue
		}
		if cap.Valid {
			k.Capsule = cap.String
		}
		o.Keys = append(o.Keys, k)
	}
	return nil
}

// populateMyKeys fills in only the keys for a specific agent
func (o *Operation) populateMyKeys(ctx context.Context, gid GoogleID) error {
	rows, err := db.QueryContext(ctx, "SELECT portalID, onhand, capsule FROM opkeys WHERE opID = ? AND gid = ?", o.ID, gid)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var k KeyOnHand
		k.Gid = gid
		var cap sql.NullString
		if err := rows.Scan(&k.ID, &k.Onhand, &cap); err != nil {
			continue
		}
		if cap.Valid {
			k.Capsule = cap.String
		}
		o.Keys = append(o.Keys, k)
	}
	return nil
}

// KeyOnHand updates a user's key-count via the public API
func (o *Operation) KeyOnHand(ctx context.Context, gid GoogleID, portalID PortalID, count int32, capsule string) error {
	k := KeyOnHand{
		ID:      portalID,
		Gid:     gid,
		Onhand:  count,
		Capsule: capsule,
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := o.insertKey(ctx, k, tx); err != nil {
		return err
	}

	return tx.Commit()
}
