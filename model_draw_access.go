package wasabee

import (
	"database/sql"
	"fmt"
)


func (o *Operation) PopulateTeams() error {
	if len(o.Teams) > 0 {
		return nil
	}

	// MIGRATE OLD STYLE TO NEW
	var teamID TeamID
	err := db.QueryRow("SELECT teamID FROM operation WHERE ID = ?", o.ID).Scan(&teamID)
	if err != nil {
		Log.Notice(err)
		return err
	}
        var teamSet = 0;
	err = db.QueryRow("SELECT COUNT(*) FROM opteams WHERE opID = ? AND teamID = ? and permission = 'read'", o.ID, teamID).Scan(&teamSet)
	if err != nil {
		Log.Notice(err)
		return err
	}
	if teamID != "unused" && teamSet == 0 {
		// not migrated yet
		// can't use o.AddPerm since that requires the GID which might not be good in this case
		Log.Errorf("migrating team %s for op %s", teamID, o.ID)
		_, err = db.Exec("INSERT INTO opteams VALUES (?,?,?)", teamID, o.ID, "read")
		if err != nil {
			Log.Notice(err)
			return err
		}
		// _, err = db.Exec("UPDATE operation SET teamID = 'unused' WHERE ID = ?", o.ID)
		// if err != nil {
		//	Log.Notice(err)
		//	return err
		//}
	}

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
		o.PopulateTeams()
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
	if len(o.Teams) == 0 {
		o.PopulateTeams()
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

// Chgrp changes an operation's team -- because UNIX libc function names are cool, yo
func (opID OperationID) ChgrpDeprecated(gid GoogleID, to TeamID) error {
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

func (o *Operation) AssignedOnlyAccess(gid GoogleID) bool {
	if len(o.Teams) == 0 {
		o.PopulateTeams()
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

	// XXX validate perm. invalid values will store as ""
	// make sure it is not already there?
	_, err = db.Exec("INSERT INTO opteams VALUES (?,?,?)", teamID, o.ID, perm)
	if err != nil {
		Log.Error(err)
		return err
	}
	// XXX MIGRATION PATH
	_, err = db.Exec("UPDATE operation SET teamID = ? WHERE ID = ?", teamID, o.ID)

	return nil
}

func (o *Operation) DelPerm(gid GoogleID, teamID TeamID, perm string) error {
	if !o.ID.IsOwner(gid) {
		err := fmt.Errorf("%s not current owner of op %s", gid, o.ID)
		Log.Error(err)
		return err
	}

	// XXX this will get multiples if they were added
	_, err := db.Exec("DELETE FROM opteams WHERE teamID = ? AND opID = ? AND permission = ?", teamID, o.ID, perm)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}
