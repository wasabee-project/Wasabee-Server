package model

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/util"
)

// KeyOnHand describes the already in possession for the op
type KeyOnHand struct {
	ID      PortalID `json:"portalId"`
	Gid     GoogleID `json:"gid"`
	Capsule string   `json:"capsule"`
	Onhand  int32    `json:"onhand"`
}

// insertKey adds a user keycount to the database
func (o *Operation) insertKey(k KeyOnHand, tx *sql.Tx) error {
	details, err := o.ID.portalDetails(k.ID, tx)
	if err != nil {
		log.Error(err.Error())
		return err
	}
	if details.Name == "" {
		log.Infow("attempt to assign key count to portal not in op", "GID", k.Gid, "resource", o.ID, "portal", k.ID)
		if _, err = tx.Exec("DELETE FROM opkeys WHERE opID = ? AND portalID = ?", o.ID, k.ID); err != nil {
			log.Info(err)
			err := fmt.Errorf(ErrKeyUnableToRemove)
			return err
		}
		return nil
	}

	k.Capsule = util.Sanitize(k.Capsule) // can be NULL, but NULL causes the unique key to not work as intended
	if k.Onhand == 0 {
		if _, err = tx.Exec("DELETE FROM opkeys WHERE opID = ? AND portalID = ? AND gid = ? AND capsule = ?", o.ID, k.ID, k.Gid, k.Capsule); err != nil {
			log.Info(err)
			err := fmt.Errorf(ErrKeyUnableToRemove)
			return err
		}
	} else {
		_, err = tx.Exec("REPLACE INTO opkeys (opID, portalID, gid, onhand, capsule) VALUES (?, ?, ?, ?, ?)", o.ID, k.ID, k.Gid, k.Onhand, k.Capsule) // REPLACE OK SCB
		if err != nil && strings.Contains(err.Error(), "Error 1452") {
			log.Info(err)
			return fmt.Errorf(ErrKeyUnableToRecord)
		}
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return nil
}

// PopulateKeys fills in the Keys on hand list for the Operation. No authorization takes place.
// TODO: filter based on zones
func (o *Operation) populateKeys() error {
	var k KeyOnHand
	rows, err := db.Query("SELECT portalID, gid, onhand, capsule FROM opkeys WHERE opID = ?", o.ID)
	if err != nil {
		log.Error(err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cap sql.NullString
		err := rows.Scan(&k.ID, &k.Gid, &k.Onhand, &cap)
		if err != nil {
			log.Error(err)
			continue
		}
		if cap.Valid {
			k.Capsule = cap.String
		} else {
			k.Capsule = ""
		}
		o.Keys = append(o.Keys, k)
	}
	return nil
}

// PopulateKeys fills in the Keys on hand list for the Operation. No authorization takes place.
func (o *Operation) populateMyKeys(gid GoogleID) error {
	var k KeyOnHand
	k.Gid = gid

	rows, err := db.Query("SELECT portalID, onhand, capsule FROM opkeys WHERE opID = ? AND gid = ?", o.ID, gid)
	if err != nil {
		log.Error(err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cap sql.NullString
		err := rows.Scan(&k.ID, &k.Onhand, &cap)
		if err != nil {
			log.Error(err)
			continue
		}
		if cap.Valid {
			k.Capsule = cap.String
		} else {
			k.Capsule = ""
		}
		o.Keys = append(o.Keys, k)
	}
	return nil
}

// KeyOnHand updates a user's key-count for linking
func (o *Operation) KeyOnHand(gid GoogleID, portalID PortalID, count int32, capsule string) error {
	k := KeyOnHand{
		ID:      portalID,
		Gid:     gid,
		Onhand:  count,
		Capsule: capsule,
	}

	// get ctx from request?
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		log.Error(err)
		return err
	}
	defer func() {
		err := tx.Rollback()
		if err != nil && err != sql.ErrTxDone {
			log.Error(err)
		}
	}()

	if err := o.insertKey(k, tx); err != nil {
		log.Error(err)
		return err
	}

	if err := tx.Commit(); err != nil {
		log.Error(err)
		return err
	}
	return nil
}
