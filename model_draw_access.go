package wasabee

import (
	"database/sql"
	"fmt"
)

// PopulateTeams loads the permissions from the database into the op data
func (o *Operation) PopulateTeams() error {
	// start empty, trust only what is in the database
	o.Teams = nil

	rows, err := db.Query("SELECT teamID, permission FROM opteams WHERE opID = ?", o.ID)
	if err != nil && err != sql.ErrNoRows {
		Log.Notice(err)
		return err
	}
	defer rows.Close()

	var tid, role string
	for rows.Next() {
		err := rows.Scan(&tid, &role)
		if err != nil {
			Log.Notice(err)
			continue
		}
		o.Teams = append(o.Teams, ExtendedTeam{
			TeamID: TeamID(tid),
			Role:   etRole(role),
		})
	}
	return nil
}

// ReadAccess determines if an agent has read acces to an op
func (o *Operation) ReadAccess(gid GoogleID) bool {
	if len(o.Teams) == 0 {
		if err := o.PopulateTeams(); err != nil {
			Log.Notice(err)
			return false
		}
	}
	if o.ID.IsOwner(gid) {
		return true
	}
	for _, t := range o.Teams {
		if t.Role == etRoleAssignedOnly {
			continue
		}
		if inteam, _ := gid.AgentInTeam(t.TeamID, false); inteam {
			return true
		}
	}
	return false
}

// WriteAccess determines if an agent has write access to an op
func (o *Operation) WriteAccess(gid GoogleID) bool {
	// do not cache -- force reset on uploads
	if err := o.PopulateTeams(); err != nil {
		Log.Notice(err)
		return false
	}
	if o.ID.IsOwner(gid) {
		return true
	}
	for _, t := range o.Teams {
		if t.Role != etRoleWrite {
			continue
		}
		if inteam, _ := gid.AgentInTeam(t.TeamID, false); inteam {
			return true
		}
	}
	return false
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

	if x, err := togid.IngressName(); x == "" || err != nil {
		err := fmt.Errorf("unknown user: %s", to)
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

// AssignedOnlyAccess verifies if an agent has AO access to an op
func (o *Operation) AssignedOnlyAccess(gid GoogleID) bool {
	if len(o.Teams) == 0 {
		if err := o.PopulateTeams(); err != nil {
			Log.Error(err)
			return false
		}
	}

	for _, t := range o.Teams {
		if t.Role != etRoleAssignedOnly {
			continue
		}
		if inteam, _ := gid.AgentInTeam(t.TeamID, false); inteam {
			return true
		}
	}
	return false
}

// AddPerm adds a new permission to an op
func (o *Operation) AddPerm(gid GoogleID, teamID TeamID, perm string) error {
	if !o.ID.IsOwner(gid) {
		err := fmt.Errorf("%s not current owner of op %s", gid, o.ID)
		Log.Error(err)
		return err
	}

	inteam, err := gid.AgentInTeam(teamID, false)
	if err != nil {
		Log.Error(err)
		return err
	}
	if !inteam {
		err := fmt.Errorf("you must be on a team to add it as a permission: %s %s %s", gid, teamID, o.ID)
		Log.Error(err)
		return err
	}

	et := etRole(perm)
	err = et.isValid()
	if err != nil {
		Log.Error(err)
		return err
	}
	_, err = db.Exec("INSERT INTO opteams VALUES (?,?,?)", teamID, o.ID, perm)
	if err != nil {
		Log.Error(err)
		return err
	}

	if err = o.Touch(); err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// DelPerm removes a permission from an op
func (o *Operation) DelPerm(gid GoogleID, teamID TeamID, perm string) error {
	if !o.ID.IsOwner(gid) {
		err := fmt.Errorf("%s not current owner of op %s", gid, o.ID)
		Log.Error(err)
		return err
	}

	_, err := db.Exec("DELETE FROM opteams WHERE teamID = ? AND opID = ? AND permission = ? LIMIT 1", teamID, o.ID, perm)
	if err != nil {
		Log.Error(err)
		return err
	}
	if err = o.Touch(); err != nil {
		Log.Error(err)
		return err
	}
	return nil
}
