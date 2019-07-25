package wasabee

import (
	"database/sql"
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
}

// insertLink adds a link to the database
func (o *Operation) insertLink(l Link) error {
	if l.To == l.From {
		Log.Debug("source and destination the same, ignoring link")
		return nil
	}

	_, err := db.Exec("INSERT INTO link (ID, fromPortalID, toPortalID, opID, description, gid, throworder, completed) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		l.ID, l.From, l.To, o.ID, MakeNullString(l.Desc), MakeNullString(l.AssignedTo), l.ThrowOrder, l.Completed)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// PopulateLinks fills in the Links list for the Operation. No authorization takes place.
func (o *Operation) PopulateLinks() error {
	var tmpLink Link
	var description, gid, iname sql.NullString

	rows, err := db.Query("SELECT l.ID, l.fromPortalID, l.toPortalID, l.description, l.gid, l.throworder, l.completed, a.iname FROM link=l LEFT JOIN agent=a ON l.gid=a.gid WHERE l.opID = ? ORDER BY l.throworder", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmpLink.ID, &tmpLink.From, &tmpLink.To, &description, &gid, &tmpLink.ThrowOrder, &tmpLink.Completed, &iname)
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
		o.Links = append(o.Links, tmpLink)
	}
	return nil
}

// String returns the string version of a LinkID
func (l LinkID) String() string {
	return string(l)
}

// AssignLink assigns a link to an agent, sending them a message that they have an assignment
func (opID OperationID) AssignLink(linkID LinkID, gid GoogleID) error {
	_, err := db.Exec("UPDATE link SET gid = ? WHERE ID = ? AND opID = ?", MakeNullString(gid), linkID, opID)
	if err != nil {
		Log.Error(err)
		return err
	}

	/*
		link := struct {
			OpID   OperationID
			LinkID LinkID
		}{
			OpID:   opID,
			LinkID: linkID,
		}

		msg, err := gid.ExecuteTemplate("assignLink", link)
		if err != nil {
			Log.Error(err)
			msg = fmt.Sprintf("assigned a marker for op %s", opID)
			// do not report send errors up the chain, just log
		}
		if string(gid) != "" {
			_, err = gid.SendMessage(msg)
			if err != nil {
				Log.Error(err)
				// do not report send errors up the chain, just log
			}
		}
	*/

	if err = opID.Touch(); err != nil {
		Log.Error(err)
	}

	return nil
}

// LinkDescription updates the description for a link
func (opID OperationID) LinkDescription(linkID LinkID, desc string) error {
	_, err := db.Exec("UPDATE link SET description = ? WHERE ID = ? AND opID = ?", MakeNullString(desc), linkID, opID)
	if err != nil {
		Log.Error(err)
		return err
	}
	if err = opID.Touch(); err != nil {
		Log.Error(err)
	}
	return nil
}

// LinkCompleted updates the completed flag for a link
func (opID OperationID) LinkCompleted(linkID LinkID, completed bool) error {
	_, err := db.Exec("UPDATE link SET completed = ? WHERE ID = ? AND opID = ?", completed, linkID, opID)
	if err != nil {
		Log.Error(err)
		return err
	}
	if err = opID.Touch(); err != nil {
		Log.Error(err)
	}
	return nil
}

func (opID OperationID) AssignedTo(link LinkID, gid GoogleID) bool {
	var x int

	err := db.QueryRow("SELECT COUNT(*) FROM LINK WHERE opID = ? AND ID = ? AND gid = ?", opID, link, gid).Scan(&x)
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
func (opID OperationID) LinkOrder(order string, gid GoogleID) error {
	// check isowner (already done in http/pdraw.go, but there may be other callers in the future

	stmt, err := db.Prepare("UPDATE link SET throworder = ? WHERE opID = ? AND ID = ?")
	if err != nil {
		Log.Error(err)
		return err
	}

	pos := 1
	links := strings.Split(order, ",")
	for i := range links {
		if links[i] == "000" { // the header, could be anyplace in the order if the user was being silly
			continue
		}
		if _, err := stmt.Exec(pos, opID, links[i]); err != nil {
			Log.Error(err)
			continue
		}
		pos++
	}
	if err = opID.Touch(); err != nil {
		Log.Error(err)
	}
	return nil
}

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

func MinPortalLevel(distance float64, agents int, allowmods bool) float64 {
	if distance < 160.0 {
		return 1.000
	}
	if distance > 6553000.0 {
		// link amp required
		return 8.0
	}

	m := (root(distance, 4)) / (2 * root(10, 4))
	return m
}

func root(a float64, n int) float64 {
	n1 := n - 1
	n1f, rn := float64(n1), 1/float64(n)
	x, x0 := 1., 0.
	for {
		potx, t2 := 1/x, a
		for b := n1; b > 0; b >>= 1 {
			if b&1 == 1 {
				t2 *= potx
			}
			potx *= potx
		}
		x0, x = x, rn*(n1f*x+t2)
		if math.Abs(x-x0)*1e15 < x {
			break
		}
	}
	return x
}
