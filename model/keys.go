package model

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/wasabee-project/Wasabee-Server/log"
)

// KeyOnHand describes the already in possession for the op
type KeyOnHand struct {
	ID      PortalID `json:"portalId"`
	Gid     GoogleID `json:"gid"`
	Onhand  int32    `json:"onhand"`
	Capsule string   `json:"capsule"`
}

// insertKey adds a user keycount to the database
func (o *Operation) insertKey(k KeyOnHand) error {
	details, err := o.PortalDetails(k.ID, k.Gid)
	if err != nil {
		log.Error(err.Error())
		return err
	}
	if details.Name == "" {
		log.Infow("attempt to assign key count to portal not in op", "GID", k.Gid, "resource", o.ID, "portal", k.ID)
		return nil
	}

	if k.Onhand == 0 {
		if _, err = db.Exec("DELETE FROM opkeys WHERE opID = ? AND portalID = ? and gid = ?", o.ID, k.ID, k.Gid); err != nil {
			log.Info(err)
			err := fmt.Errorf("unable to remove key count for portal")
			return err
		}
	} else {
		_, err = db.Exec("INSERT INTO opkeys (opID, portalID, gid, onhand, capsule) VALUES (?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE onhand = ?, capsule = ?",
			o.ID, k.ID, k.Gid, k.Onhand, MakeNullString(k.Capsule), k.Onhand, MakeNullString(k.Capsule))
		if err != nil && strings.Contains(err.Error(), "Error 1452") {
			log.Info(err)
			err := fmt.Errorf("unable to record keys, ensure the op on the server is up-to-date")
			return err
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

	return o.insertKey(k)
}