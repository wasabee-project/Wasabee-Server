package wasabee

import (
	"database/sql"
	"fmt"
)

// getTeamID returns the teamID for an op
func (opID OperationID) getTeamIDdeprecated() (TeamID, error) {
	var teamID TeamID
	err := db.QueryRow("SELECT teamID FROM operation WHERE ID = ?", opID).Scan(&teamID)
	if err != nil && err != sql.ErrNoRows {
		Log.Notice(err)
		return "", err
	}
	if err != nil && err == sql.ErrNoRows {
		return "", nil
	}
	return teamID, nil
}

// PopulateTeams fills in the Teams data for an Operation
func (o *Operation) PopulateTeams() error {
	if len(o.Teams) > 0 {
		return nil
	}

	primaryTeam, _ := o.ID.getTeamIDdeprecated()
	o.Teams = append(o.Teams, ExtendedTeam{
		TeamID: primaryTeam,
		Role:   etRoleRead,
	})

	rows, err := db.Query("SELECT teamID, role FROM opteams WHERE opID = ?", o.ID)
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
	if o.ID.IsOwner(gid) {
		return true
	}
	if len(o.Teams) == 0 {
		o.PopulateTeams()
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
	if o.ID.IsOwner(gid) {
		return true
	}
	if len(o.Teams) == 0 {
		o.PopulateTeams()
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
