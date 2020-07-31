package wasabee

import (
	"database/sql"
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
	_, err := db.Exec("INSERT INTO opkeys (opID, portalID, gid, onhand, capsule) VALUES (?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE onhand = ?, capsule = ?",
		o.ID, k.ID, k.Gid, k.Onhand, MakeNullString(k.Capsule), k.Onhand, MakeNullString(k.Capsule))
	if err != nil {
		Log.Error(err)
		return err
	}
	if err = o.Touch(); err != nil {
		Log.Error(err)
	}
	return nil
}

// PopulateKeys fills in the Keys on hand list for the Operation. No authorization takes place.
func (o *Operation) populateKeys() error {
	var k KeyOnHand
	rows, err := db.Query("SELECT portalID, gid, onhand, capsule FROM opkeys WHERE opID = ?", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rows.Close()

	var cap sql.NullString
	for rows.Next() {
		err := rows.Scan(&k.ID, &k.Gid, &k.Onhand, &cap)
		if err != nil {
			Log.Error(err)
			continue
		}
		if cap.Valid {
			k.Capsule = cap.String
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
	if err := o.insertKey(k); err != nil {
		Log.Error(err)
		return err
	}
	return nil
}
