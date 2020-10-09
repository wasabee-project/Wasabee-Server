package wasabee

import (
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// LinkID wrapper to ensure type safety
type LinkID string

// Link is defined by the Wasabee IITC plugin.
type Link struct {
	ID         LinkID   `json:"ID"`
	From       PortalID `json:"fromPortalId"`
	To         PortalID `json:"toPortalId"`
	Desc       string   `json:"description"`
	AssignedTo GoogleID `json:"assignedTo"`
	Iname      string   `json:"assignedToNickname"`
	ThrowOrder int32    `json:"throwOrderPos"`
	Completed  bool     `json:"completed"`
	Color      string   `json:"color"`
	Zone       Zone     `json:"zone"`
}

// insertLink adds a link to the database
func (opID OperationID) insertLink(l Link) error {
	if l.To == l.From {
		Log.Infow("source and destination the same, ignoring link", "resource", opID)
		return nil
	}

	if !l.Zone.Valid() || l.Zone == ZoneAll {
		l.Zone = zonePrimary
	}

	_, err := db.Exec("INSERT INTO link (ID, fromPortalID, toPortalID, opID, description, gid, throworder, completed, color, zone) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		l.ID, l.From, l.To, opID, MakeNullString(l.Desc), MakeNullString(l.AssignedTo), l.ThrowOrder, l.Completed, l.Color, l.Zone)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

func (opID OperationID) deleteLink(lid LinkID) error {
	_, err := db.Exec("DELETE FROM link WHERE OpID = ? and ID = ?", opID, lid)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

func (opID OperationID) updateLink(l Link) error {
	if l.To == l.From {
		Log.Infow("source and destination the same, ignoring link", "resource", opID)
		return nil
	}

	if !l.Zone.Valid() || l.Zone == ZoneAll {
		l.Zone = zonePrimary
	}

	_, err := db.Exec("INSERT INTO link (ID, fromPortalID, toPortalID, opID, description, gid, throworder, completed, color, zone) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE fromPortalID = ?, toPortalID = ?, description = ?, color=?, zone = ?, gid = ?, completed = ?",
		l.ID, l.From, l.To, opID, MakeNullString(l.Desc), MakeNullString(l.AssignedTo), l.ThrowOrder, l.Completed, l.Color, l.Zone,
		l.From, l.To, MakeNullString(l.Desc), l.Color, l.Zone, MakeNullString(l.AssignedTo), l.Completed)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// PopulateLinks fills in the Links list for the Operation. No authorization takes place.
func (o *Operation) populateLinks(zones []Zone, inGid GoogleID) error {
	var tmpLink Link
	var description, gid, iname sql.NullString

	var err error
	var rows *sql.Rows
	rows, err = db.Query("SELECT l.ID, l.fromPortalID, l.toPortalID, l.description, l.gid, l.throworder, l.completed, a.iname, l.color, l.zone FROM link=l LEFT JOIN agent=a ON l.gid=a.gid WHERE l.opID = ? ORDER BY l.throworder", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmpLink.ID, &tmpLink.From, &tmpLink.To, &description, &gid, &tmpLink.ThrowOrder, &tmpLink.Completed, &iname, &tmpLink.Color, &tmpLink.Zone)
		if err != nil {
			Log.Error(err)
			continue
		}
		if description.Valid {
			tmpLink.Desc = description.String
		} else {
			tmpLink.Desc = ""
		}
		if gid.Valid {
			tmpLink.AssignedTo = GoogleID(gid.String)
		} else {
			tmpLink.AssignedTo = ""
		}
		if iname.Valid {
			tmpLink.Iname = iname.String
		} else {
			tmpLink.Iname = ""
		}
		// this isn't in a zone with which we are concerned AND not assigned to me, skip
		if !tmpLink.Zone.inZones(zones) && tmpLink.AssignedTo != inGid {
			continue
		}
		o.Links = append(o.Links, tmpLink)
	}
	return nil
}

// String returns the string version of a LinkID
func (l LinkID) String() string {
	return string(l)
}

// AssignLink assigns a link to an agent
func (o *Operation) AssignLink(linkID LinkID, gid GoogleID) (string, error) {
	// gid of 0 unsets the assignment
	if gid == "0" {
		gid = ""
	}

	result, err := db.Exec("UPDATE link SET gid = ? WHERE ID = ? AND opID = ?", MakeNullString(gid), linkID, o.ID)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	ra, _ := result.RowsAffected()
	if ra != 1 {
		Log.Debugw("AssignLink rows changed", "rows", ra, "resource", o.ID, "GID", gid, "link", linkID)
		return "", nil
	}

	if gid != "" {
		o.ID.firebaseAssignLink(gid, linkID)
	}
	return o.Touch()
}

// LinkDescription updates the description for a link
func (o *Operation) LinkDescription(linkID LinkID, desc string) (string, error) {
	_, err := db.Exec("UPDATE link SET description = ? WHERE ID = ? AND opID = ?", MakeNullString(desc), linkID, o.ID)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	return o.Touch()
}

// LinkCompleted updates the completed flag for a link
func (o *Operation) LinkCompleted(linkID LinkID, completed bool) (string, error) {
	_, err := db.Exec("UPDATE link SET completed = ? WHERE ID = ? AND opID = ?", completed, linkID, o.ID)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	o.firebaseLinkStatus(linkID, completed)
	return o.Touch()
}

// AssignedTo checks to see if a link is assigned to a particular agent
func (opID OperationID) AssignedTo(link LinkID, gid GoogleID) bool {
	var x int

	err := db.QueryRow("SELECT COUNT(*) FROM link WHERE opID = ? AND ID = ? AND gid = ?", opID, link, gid).Scan(&x)
	if err != nil {
		Log.Error(err)
		return false
	}
	if x != 1 {
		return false
	}
	return true
}

// LinkOrder changes the order of the throws for an operation
func (o *Operation) LinkOrder(order string, gid GoogleID) (string, error) {
	// check isowner (already done in http/pdraw.go, but there may be other callers in the future

	stmt, err := db.Prepare("UPDATE link SET throworder = ? WHERE opID = ? AND ID = ?")
	if err != nil {
		Log.Error(err)
		return "", err
	}

	pos := 1
	links := strings.Split(order, ",")
	for i := range links {
		if links[i] == "000" { // the header, could be anyplace in the order if the user was being silly
			continue
		}
		if _, err := stmt.Exec(pos, o.ID, links[i]); err != nil {
			Log.Error(err)
			continue
		}
		pos++
	}
	return o.Touch()
}

// LinkColor changes the color of a link in an operation
func (o *Operation) LinkColor(link LinkID, color string) (string, error) {
	_, err := db.Exec("UPDATE link SET color = ? WHERE ID = ? and opID = ?", color, link, o.ID)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	return o.Touch()
}

// LinkSwap changes the direction of a link in an operation
func (o *Operation) LinkSwap(link LinkID) (string, error) {
	var tmpLink Link

	err := db.QueryRow("SELECT fromPortalID, toPortalID FROM link WHERE opID = ? AND ID = ?", o.ID, link).Scan(&tmpLink.From, &tmpLink.To)
	if err != nil {
		Log.Error(err)
		return "", err
	}

	_, err = db.Exec("UPDATE link SET fromPortalID = ?, toPortalID = ? WHERE ID = ? and opID = ?", tmpLink.To, tmpLink.From, link, o.ID)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	return o.Touch()
}

// Distance calculates the distance between to lat/long pairs
func Distance(startLat, startLon, endLat, endLon string) float64 {
	sl, _ := strconv.ParseFloat(startLat, 64)
	startrl := math.Pi * sl / 180.0
	el, _ := strconv.ParseFloat(endLat, 64)
	endrl := math.Pi * el / 180.0

	t, _ := strconv.ParseFloat(startLon, 64)
	th, _ := strconv.ParseFloat(endLon, 64)
	rt := math.Pi * (t - th) / 180.0

	dist := math.Sin(startrl)*math.Sin(endrl) + math.Cos(startrl)*math.Cos(endrl)*math.Cos(rt)
	if dist > 1 {
		dist = 1
	}

	dist = math.Acos(dist)
	dist = dist * 180 / math.Pi
	dist = dist * 60 * 1.1515 * 1.609344
	return dist * 1000
}

// MinPortalLevel calculates the minimum portal level to make a link.
// It needs to be extended to calculate required mods
func MinPortalLevel(distance float64, agents int, allowmods bool) float64 {
	if distance < 160.0 {
		return 1.000
	}
	if distance > 6553000.0 {
		// link amp required
		return 8.0
	}

	m := (fourthroot(distance)) / (2 * fourthroot(10))
	return m
}

func fourthroot(a float64) float64 {
	return math.Pow(math.E, math.Log(a)/4.0)
}

// lookup and return a populated link from an ID
func (o *Operation) getLink(linkID LinkID) (Link, error) {
	for _, l := range o.Links {
		if l.ID == linkID {
			return l, nil
		}
	}

	var l Link
	err := fmt.Errorf("link not found")
	return l, err
}

// SetZone sets a link's zone -- caller must authorize
func (l LinkID) SetZone(o *Operation, z Zone) (string, error) {
	if _, err := db.Exec("UPDATE link SET zone = ? WHERE ID = ? AND opID = ?", z, l, o.ID); err != nil {
		Log.Error(err)
		return "", err
	}
	return o.Touch()
}
