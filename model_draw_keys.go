package wasabee

// KeyOnHand describes the already in possession for the op
type KeyOnHand struct {
	ID     PortalID `json:"portalId"`
	Gid    GoogleID `json:"gid"`
	Onhand int32    `json:"onhand"`
}

// insertKey adds a user keycount to the database
func (o *Operation) insertKey(k KeyOnHand) error {
	_, err := db.Exec("INSERT INTO opkeys (opID, portalID, gid, onhand) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE onhand = ?",
		o.ID, k.ID, k.Gid, k.Onhand, k.Onhand)
	if err != nil {
		Log.Error(err)
		return err
	}
	if err = o.ID.Touch(); err != nil {
		Log.Error(err)
	}
	return nil
}

// PopulateKeys fills in the Keys on hand list for the Operation. No authorization takes place.
func (o *Operation) PopulateKeys() error {
	var k KeyOnHand
	rows, err := db.Query("SELECT portalID, gid, onhand FROM opkeys WHERE opID = ?", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&k.ID, &k.Gid, &k.Onhand)
		if err != nil {
			Log.Error(err)
			continue
		}
		o.Keys = append(o.Keys, k)
	}
	return nil
}

// KeyOnHand updates a user's key-count for linking
func (opID OperationID) KeyOnHand(gid GoogleID, portalID PortalID, count int32) error {
	var o Operation
	o.ID = opID
	k := KeyOnHand{
		ID:     portalID,
		Gid:    gid,
		Onhand: count,
	}
	if err := o.insertKey(k); err != nil {
		Log.Error(err)
		return err
	}
	return nil
}
