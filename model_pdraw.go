package wasabi

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"sort"
	"strconv"
)

// OperationID wrapper to ensure type safety
type OperationID string

// PortalID wrapper to ensure type safety
type PortalID string

// MarkerType will be an enum once we figure out the full list
type MarkerType string

// Operation is defined by the PhtivDraw IITC plugin.
// It is the top level item in the JSON file.
type Operation struct {
	ID        OperationID `json:"ID"`
	Name      string      `json:"name"`
	Gid       GoogleID    `json:"creator"` // IITC plugin sending agent name, need to convert to GID
	Color     string      `json:"color"`   // could be an enum, but freeform is fine for now
	OpPortals []Portal    `json:"opportals"`
	Anchors   []PortalID  `json:"anchors"`
	Links     []Link      `json:"links"`
	Markers   []Marker    `json:"markers"`
	TeamID    TeamID      `json:"teamid"`
	Modified  string      `json:"modified"`
}

// Portal is defined by the PhtivDraw IITC plugin.
type Portal struct {
	ID       PortalID `json:"id"`
	Name     string   `json:"name"`
	Lat      string   `json:"lat"` // passing these as strings saves me parsing them
	Lon      string   `json:"lng"`
	Comment  string   `json:"comment"`
	Hardness string   `json:"hardness"` // string for now, enum in the future
}

// Link is defined by the PhtivDraw IITC plugin.
type Link struct {
	ID         string   `json:"ID"`
	From       PortalID `json:"fromPortalId"`
	To         PortalID `json:"toPortalId"`
	Desc       string   `json:"description"`
	AssignedTo GoogleID `json:"assignedTo"`
	ThrowOrder float64  `json:"throwOrderPos"` // currently not in database, need schema change
}

// Marker is defined by the PhtivDraw IITC plugin.
type Marker struct {
	ID         string     `json:"ID"`
	PortalID   PortalID   `json:"portalId"`
	Type       MarkerType `json:"type"`
	Comment    string     `json:"comment"`
	AssignedTo GoogleID   `json:"assignedTo"` // currently not in database, need schema change
}

// PDrawInsert parses a raw op sent from the IITC plugin and stores it in the database
// it will completely overwrite an existing draw with the same ID
// if the current agent is the same as the agent who originally uploaded it
func PDrawInsert(op json.RawMessage, gid GoogleID) error {
	var o Operation
	if err := json.Unmarshal(op, &o); err != nil {
		Log.Error(err)
		return err
	}

	_, teamID, err := pdrawAuthorized(gid, o.ID)
	if err != nil { // !authorized always sets error
		Log.Error(err)
		return err
	}

	// clear and start from a blank slate
	if err = o.Delete(); err != nil {
		Log.Error(err)
		return err
	}

	// create a new team if one did not already exist
	if teamID.String() == "" {
		teamID, err = gid.NewTeam(o.Name)
		if err != nil {
			Log.Error(err)
		}
	}

	// start the insert process
	_, err = db.Exec("INSERT INTO operation (ID, name, gid, color, teamID, modified) VALUES (?, ?, ?, ?, ?, NOW())", o.ID, o.Name, gid, o.Color, teamID.String())
	if err != nil {
		Log.Error(err)
		return err
	}

	for _, m := range o.Markers {
		if err = o.insertMarker(m); err != nil {
			Log.Error(err)
			continue
		}
	}
	for _, l := range o.Links {
		if err = o.insertLink(l); err != nil {
			Log.Error(err)
			continue
		}
	}
	for _, a := range o.Anchors {
		if err = o.insertAnchor(a); err != nil {
			Log.Error(err)
			continue
		}
	}

	// I bet this isn't needed since they should be covered in links and markers... but just in case
	for _, p := range o.OpPortals {
		if err = o.insertPortal(p); err != nil {
			Log.Error(err)
			continue
		}
	}
	return nil
}

// PDrawUpdate - probably redundant as long as the id does not change?
func PDrawUpdate(id string, op json.RawMessage, gid GoogleID) error {
	var o Operation
	if err := json.Unmarshal(op, &o); err != nil {
		Log.Error(err)
		return err
	}

	if id != string(o.ID) {
		err := fmt.Errorf("incoming op.ID does not match the URL specified ID: refusing update")
		Log.Error(err)
		return err
	}

	return PDrawInsert(op, gid)
}

func pdrawAuthorized(gid GoogleID, oid OperationID) (bool, TeamID, error) {
	var opgid GoogleID
	var teamID TeamID
	var authorized bool
	err := db.QueryRow("SELECT gid, teamID FROM operation WHERE ID = ?", oid).Scan(&opgid, &teamID)
	if err != nil && err != sql.ErrNoRows {
		Log.Notice(err)
		return false, "", err
	}
	if err != nil && err == sql.ErrNoRows {
		authorized = true
	}
	if opgid == gid {
		authorized = true
	}
	if !authorized {
		return false, teamID, errors.New("unauthorized: this operation owned by someone else")
	}
	return authorized, teamID, nil
}

// insertMarkers adds a marker to the database
func (o *Operation) insertMarker(m Marker) error {
	_, err := db.Exec("INSERT INTO marker (ID, opID, PortalID, type, gid, comment) VALUES (?, ?, ?, ?, ?, ?)",
		m.ID, o.ID, m.PortalID, m.Type, m.AssignedTo, m.Comment)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// insertPortal adds a portal to the database
func (o *Operation) insertPortal(p Portal) error {
	_, err := db.Exec("INSERT IGNORE INTO portal (ID, opID, name, loc, comment, hardness) VALUES (?, ?, ?, POINT(?, ?), ?, ?)",
		p.ID, o.ID, p.Name, p.Lon, p.Lat, p.Comment, p.Hardness)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// insertAnchor adds an anchor to the database
func (o *Operation) insertAnchor(p PortalID) error {
	_, err := db.Exec("INSERT IGNORE INTO anchor (opID, portalID) VALUES (?, ?)", o.ID, p)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// insertLink adds a link to the database
func (o *Operation) insertLink(l Link) error {
	_, err := db.Exec("INSERT INTO link (ID, fromPortalID, toPortalID, opID, description, gid, throworder) VALUES (?, ?, ?, ?, ?, ?, ?)",
		l.ID, l.From, l.To, o.ID, l.Desc, l.AssignedTo, l.ThrowOrder)
	if err != nil {
		Log.Error(err)
	}
	return nil
}

// Delete removes an operation and all associated data
func (o *Operation) Delete() error {
	_, err := db.Exec("DELETE FROM operation WHERE ID = ?", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	// the foreign key constraints should take care of these, but just in case...
	_, _ = db.Exec("DELETE FROM marker WHERE opID = ?", o.ID)
	_, _ = db.Exec("DELETE FROM link WHERE opID = ?", o.ID)
	_, _ = db.Exec("DELETE FROM portal WHERE opID = ?", o.ID)
	_, _ = db.Exec("DELETE FROM anchor WHERE opID = ?", o.ID)
	return nil
}

// Populate takes a pointer to an Operation and fills it in; o.ID must be set
// checks to see that either the gid created the operation or the gid is on the team assigned to the operation
func (o *Operation) Populate(gid GoogleID) error {
	var authorized bool

	// permission check and populate Operation top level
	var teamID sql.NullString
	r := db.QueryRow("SELECT name, gid, color, teamID, modified FROM operation WHERE ID = ?", o.ID)
	err := r.Scan(&o.Name, &o.Gid, &o.Color, &teamID, &o.Modified)
	if err != nil {
		Log.Error(err)
		return err
	}
	if teamID.Valid {
		o.TeamID = TeamID(teamID.String)
		if inteam, _ := gid.AgentInTeam(o.TeamID, false); inteam {
			authorized = true
		}
	}
	if gid == o.Gid {
		authorized = true
	}
	if !authorized {
		return errors.New("unauthorized: you are not on a team authorized to see this operation")
	}

	err = o.PopulatePortals()
	if err != nil {
		Log.Notice(err)
		return err
	}

	err = o.PopulateMarkers()
	if err != nil {
		Log.Notice(err)
		return err
	}

	err = o.PopulateLinks()
	if err != nil {
		Log.Notice(err)
		return err
	}

	err = o.PopulateAnchors()
	if err != nil {
		Log.Notice(err)
		return err
	}

	return nil
}

// PopulatePortals fills in the OpPortals list for the Operation. No authorization takes place.
func (o *Operation) PopulatePortals() error {
	var tmpPortal Portal

	var comment, hardness sql.NullString

	rows, err := db.Query("SELECT ID, name, Y(loc) AS lat, X(loc) AS lon, comment, hardness FROM portal WHERE opID = ? ORDER BY name", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmpPortal.ID, &tmpPortal.Name, &tmpPortal.Lat, &tmpPortal.Lon, &comment, &hardness)
		if err != nil {
			Log.Error(err)
			continue
		}
		if comment.Valid {
			tmpPortal.Comment = comment.String
		} else {
			tmpPortal.Comment = ""
		}
		if hardness.Valid {
			tmpPortal.Hardness = hardness.String
		} else {
			tmpPortal.Hardness = ""
		}

		o.OpPortals = append(o.OpPortals, tmpPortal)
	}
	return nil
}

// PopulateMarkers fills in the Markers list for the Operation. No authorization takes place.
func (o *Operation) PopulateMarkers() error {
	var tmpMarker Marker
	var gid, comment sql.NullString

	rows, err := db.Query("SELECT ID, PortalID, type, gid, comment FROM marker WHERE opID = ?", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmpMarker.ID, &tmpMarker.PortalID, &tmpMarker.Type, &gid, &comment)
		if err != nil {
			Log.Error(err)
			continue
		}
		if gid.Valid {
			tmpMarker.AssignedTo = GoogleID(gid.String)
		} else {
			tmpMarker.Comment = ""
		}
		if comment.Valid {
			tmpMarker.Comment = comment.String
		} else {
			tmpMarker.Comment = ""
		}
		o.Markers = append(o.Markers, tmpMarker)
	}
	return nil
}

// PopulateLinks fills in the Links list for the Operation. No authorization takes place.
func (o *Operation) PopulateLinks() error {
	var tmpLink Link
	var description, gid sql.NullString

	rows, err := db.Query("SELECT ID, fromPortalID, toPortalID, description, gid, throworder FROM link WHERE opID = ? ORDER BY throworder", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmpLink.ID, &tmpLink.From, &tmpLink.To, &description, &gid, &tmpLink.ThrowOrder)
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
		o.Links = append(o.Links, tmpLink)
	}
	return nil
}

// PopulateAnchors fills in the Anchors list for the Operation. No authorization takes place.
func (o *Operation) PopulateAnchors() error {
	var anchor PortalID
	rows, err := db.Query("SELECT portalID FROM anchor WHERE opID = ?", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&anchor)
		if err != nil {
			Log.Error(err)
			continue
		}
		o.Anchors = append(o.Anchors, anchor)
	}
	return nil
}

// this is still very early -- dunno what the client is going to want
func (teamID TeamID) pdMarkers(tl *TeamData) error {
	mr, err := db.Query("SELECT m.ID, m.portalID, m.type, m.comment, Y(p.loc) AS lat, X(p.loc) AS lon, p.name FROM marker=m, portal=p WHERE m.opID IN (SELECT ID FROM operation WHERE teamID = ?) AND m.portalID = p.ID AND m.opID = p.opID", teamID)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer mr.Close()

	var tmpMarker Marker
	var tmpWaypoint waypoint
	for mr.Next() {
		err := mr.Scan(&tmpMarker.ID, &tmpMarker.PortalID, &tmpMarker.Type, &tmpMarker.Comment, &tmpWaypoint.Lat, &tmpWaypoint.Lon, &tmpWaypoint.Desc)
		if err != nil {
			Log.Error(err)
			continue
		}
		tl.Markers = append(tl.Markers, tmpMarker)

		tmpWaypoint.Type = wpc
		tmpWaypoint.MarkerType = tmpMarker.Type.String()
		tmpWaypoint.TeamID = teamID.String()
		tmpWaypoint.ID = markerIDwaypointID(tmpMarker.ID)
		tmpWaypoint.Radius = 150
		tmpWaypoint.Share = true
		tl.Waypoints = append(tl.Waypoints, tmpWaypoint)
	}
	return nil
}

func (gid GoogleID) pdWaypoints(wc *waypointCommand) error {
	mr, err := db.Query("SELECT m.ID, m.type, Y(p.loc) AS lat, X(p.loc) AS lon, p.name FROM marker=m, portal=p WHERE m.opID IN (SELECT ID FROM operation WHERE teamID IN (SELECT t.teamID FROM agentteams=t WHERE gid = ? AND state != 'Off')) AND m.portalID = p.ID AND m.opID = p.opID", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer mr.Close()
	var markerID string
	var tmpWaypoint waypoint
	for mr.Next() {
		err := mr.Scan(&markerID, &tmpWaypoint.MarkerType, &tmpWaypoint.Lat, &tmpWaypoint.Lon, &tmpWaypoint.Desc)
		if err != nil {
			Log.Error(err)
			continue
		}
		tmpWaypoint.Type = wpc
		tmpWaypoint.ID = markerIDwaypointID(markerID)
		tmpWaypoint.Radius = 150
		tmpWaypoint.Share = true
		wc.Waypoints.Waypoints = append(wc.Waypoints.Waypoints, tmpWaypoint)
	}
	return nil
}

func (gid GoogleID) pdMarkersNear(maxdistance int, maxresults int, td *TeamData) error {
	var lat, lon string
	err := db.QueryRow("SELECT Y(loc), X(loc) FROM locations WHERE gid = ?", gid).Scan(&lat, &lon)
	if err != nil {
		Log.Error(err)
		return err
	}

	mr, err := db.Query("SELECT m.ID, m.type, Y(p.loc) AS lat, X(p.loc) AS lon, p.name, "+
		"ROUND(6371 * acos (cos(radians(?)) * cos(radians(Y(p.loc))) * cos(radians(X(p.loc)) - radians(?)) + sin(radians(?)) * sin(radians(Y(p.loc))))) AS distance "+
		"FROM marker=m, portal=p "+
		"WHERE m.opID IN (SELECT ID FROM operation WHERE teamID IN (SELECT t.teamID FROM agentteams=t WHERE gid = ? AND state != 'Off')) "+
		"AND m.portalID = p.ID AND m.opID = p.opID "+
		"HAVING distance < ? ORDER BY distance LIMIT 0,?", lat, lon, lat, gid, maxdistance, maxresults)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer mr.Close()
	var markerID string
	var tmpWaypoint waypoint
	for mr.Next() {
		err := mr.Scan(&markerID, &tmpWaypoint.MarkerType, &tmpWaypoint.Lat, &tmpWaypoint.Lon, &tmpWaypoint.Desc, &tmpWaypoint.Distance)
		if err != nil {
			Log.Error(err)
			continue
		}
		tmpWaypoint.Type = wpc
		tmpWaypoint.ID = markerIDwaypointID(markerID)
		tmpWaypoint.Radius = 150
		tmpWaypoint.Share = true
		td.Waypoints = append(td.Waypoints, tmpWaypoint)
	}

	// since otWaypoints already set, we need to resort
	sort.Slice(td.Waypoints, func(i, j int) bool {
		return td.Waypoints[i].Distance < td.Waypoints[j].Distance
	})
	return nil
}

// IsOwner returns a bool value determining if the operation is owned by the specified googleID
func (opID OperationID) IsOwner(gid GoogleID) bool {
	var c int
	err := db.QueryRow("SELECT COUNT(*) FROM operation WHERE ID = ? and gid = ?", opID, gid).Scan(&c)
	if err != nil {
		Log.Error(err)
		return false
	}
	if c < 1 {
		return false
	}
	return true
}

// IsOwner returns a bool value determining if the operation is owned by the specified googleID
func (o *Operation) IsOwner(gid GoogleID) bool {
	return o.ID.IsOwner(gid)
}

// Chown changes an operation's owner
func (opID OperationID) Chown(gid GoogleID, to string) error {
	if !opID.IsOwner(gid) {
		err := fmt.Errorf("%s not current owner of op %s", gid, opID)
		Log.Error(err)
		return err
	}

	togid, err := ToGid(to)
	if err != nil {
		Log.Error(err)
		return err
	}

	_, err = db.Exec("UPDATE operation SET gid = ? WHERE ID = ?", togid, opID)
	if err != nil {
		Log.Error(err)
		return err
	}

	return nil
}

// Chgrp changes an operation's team -- because UNIX libc function names are cool, yo
func (opID OperationID) Chgrp(gid GoogleID, to TeamID) error {
	if !opID.IsOwner(gid) {
		err := fmt.Errorf("%s not current owner of op %s", gid, opID)
		Log.Error(err)
		return err
	}

	// check to see if the team really exists
	if _, err := to.Name(); err != nil {
		Log.Error(err)
		return err
	}

	_, err := db.Exec("UPDATE operation SET teamID = ? WHERE ID = ?", to, opID)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// String returns the string version of a PortalID
func (p PortalID) String() string {
	return string(p)
}

// String returns the string version of a PortalID
func (m MarkerType) String() string {
	return string(m)
}

// markerIDwaypointID converts (hackishly) a markerID to a waypointID
// this could be a lot smarter, but we need a deterministic conversion and this works for now
func markerIDwaypointID(markerID string) int64 {
	i, _ := strconv.ParseInt("0x"+markerID[:6], 0, 64)
	return i
}

// OpUserMenu is used in html templates to draw the menus to assign targets/links
func OpUserMenu(currentGid GoogleID, teamID TeamID) (template.HTML, error) {
	rows, err := db.Query("SELECT a.iname, a.gid FROM agentteams=x, agent=a WHERE x.teamID = ? AND x.gid = a.gid ORDER BY a.iname", teamID)
	if err != nil {
		Log.Error(err)
		return "", err
	}

	defer rows.Close()

	var b bytes.Buffer
	var iname string
	var gid string

	_, _ = b.WriteString(`<select name="agent">`)
	_, _ = b.WriteString(`<option value="">-- unassigned--</option>`)
	for rows.Next() {
		err := rows.Scan(&iname, &gid)
		if err != nil {
			Log.Error(err)
			continue
		}
		if gid == string(currentGid) {
			_, _ = b.WriteString(fmt.Sprintf("<option value=\"%s\" selected=\"selected\">%s</option>", gid, iname))
		} else {
			_, _ = b.WriteString(fmt.Sprintf("<option value=\"%s\">%s</option>", gid, iname))
		}
	}
	_, _ = b.WriteString(`</select>`)
	// #nosec
	return template.HTML(b.String()), nil
}

// AssignLink assigns a link to an agent, sending them a message that they have an assignment
func (opID OperationID) AssignLink(linkID string, gid GoogleID) error {
	_, err := db.Exec("UPDATE link SET gid = ? WHERE ID = ? AND opID = ?", gid, linkID, opID)
	if err != nil {
		Log.Error(err)
		return err
	}

	// XXX get message from template
	if string(gid) != "" {
		_, err = gid.SendMessage("You have been assigned a link... better message coming eventually; something nice with a link to the op, maybe a flower.")
		if err != nil {
			Log.Error(err)
			return err
		}
	}

	return nil
}

// LinkDescription updates the description for a link
func (opID OperationID) LinkDescription(linkID, desc string) error {
	_, err := db.Exec("UPDATE link SET description = ? WHERE ID = ? AND opID = ?", desc, linkID, opID)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// AssignMarker assigns a marker to an agent, sending them a message
func (opID OperationID) AssignMarker(markerID string, gid GoogleID) error {
	_, err := db.Exec("UPDATE marker SET gid = ? WHERE ID = ? AND opID = ?", gid, markerID, opID)
	if err != nil {
		Log.Error(err)
		return err
	}

	// XXX get message from template
	_, err = gid.SendMessage("You have been assigned a target... better message coming eventually")
	if err != nil {
		Log.Error(err)
		return err
	}

	return nil
}

// MarkerComment updates the comment on a marker
func (opID OperationID) MarkerComment(markerID, comment string) error {
	_, err := db.Exec("UPDATE marker SET comment = ? WHERE ID = ? AND opID = ?", comment, markerID, opID)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// PortalHardness updates the comment on a portal
func (opID OperationID) PortalHardness(portalID PortalID, hardness string) error {
	_, err := db.Exec("UPDATE portal SET hardness = ? WHERE ID = ? AND opID = ?", hardness, portalID, opID)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// PortalComment updates the comment on a portal
func (opID OperationID) PortalComment(portalID PortalID, comment string) error {
	_, err := db.Exec("UPDATE portal SET comment = ? WHERE ID = ? AND opID = ?", comment, portalID, opID)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// PortalDetails returns information about the portal
func (opID OperationID) PortalDetails(portalID PortalID, gid GoogleID) (Portal, error) {
	var p Portal
	p.ID = portalID

	// XXX break this out into its own function, re-audit this file, I don't think operation.teamID can be NULL now
	var teamID sql.NullString
	err := db.QueryRow("SELECT teamID FROM operation WHERE ID = ?", opID).Scan(&teamID)
	if err != nil {
		Log.Error(err)
		return p, err
	}
	var inteam bool
	if teamID.Valid {
		inteam, err = gid.AgentInTeam(TeamID(teamID.String), false)
		if err != nil {
			Log.Error(err)
			return p, err
		}
	}
	if !inteam {
		err := fmt.Errorf("unauthorized: you are not on a team authorized to see this operation")
		Log.Error(err)
		return p, err
	}

	var comment, hardness sql.NullString
	err = db.QueryRow("SELECT name, Y(loc) AS lat, X(loc) AS lon, comment, hardness FROM portal WHERE opID = ? AND ID = ?", opID, portalID).Scan(&p.Name, &p.Lat, &p.Lon, &comment, &hardness)
	if err != nil {
		Log.Error(err)
		return p, err
	}
	if comment.Valid {
		p.Comment = comment.String
	}
	if hardness.Valid {
		p.Hardness = hardness.String
	}
	return p, nil
}
