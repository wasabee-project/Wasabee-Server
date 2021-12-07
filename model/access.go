package model

import (
	"database/sql"
	"fmt"

	"github.com/wasabee-project/Wasabee-Server/log"
)

// PopulateTeams loads the permissions from the database into the op data
func (o *Operation) PopulateTeams() error {
	// start empty, trust only what is in the database
	o.Teams = nil

	rows, err := db.Query("SELECT teamID, permission, zone FROM opteams WHERE opID = ?", o.ID)
	if err != nil && err != sql.ErrNoRows {
		log.Error(err)
		return err
	}

	var tid, role string
	var zone Zone
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tid, &role, &zone)
		if err != nil {
			log.Error(err)
			continue
		}
		o.Teams = append(o.Teams, OpPermission{
			OpID:   o.ID,
			TeamID: TeamID(tid),
			Role:   OpPermRole(role),
			Zone:   zone,
		})
	}
	return nil
}

// ReadAccess determines if an agent has read acces to an op, if zone limitations are present, return those as well
func (o *Operation) ReadAccess(gid GoogleID) (bool, []Zone) {
	var zones []Zone
	var permitted bool

	if o.ID.IsOwner(gid) {
		permitted = true
		zones = append(zones, ZoneAll)
		return true, zones
	}

	if len(o.Teams) == 0 {
		if err := o.PopulateTeams(); err != nil {
			log.Error(err)
			return false, zones
		}
	}

	for _, t := range o.Teams {
		switch t.Role {
		case opPermRoleAssignedOnly:
			continue
		case opPermRoleRead:
			if inteam, _ := gid.AgentInTeam(t.TeamID); inteam {
				permitted = true
				zones = append(zones, t.Zone)
				if t.Zone == ZoneAll {
					return permitted, zones // fast-path
				}
			}
		case opPermRoleWrite:
			if inteam, _ := gid.AgentInTeam(t.TeamID); inteam {
				permitted = true
				zones = append(zones, ZoneAll)
				return permitted, zones // fast-path
			}
		}
	}
	return permitted, zones
}

// WriteAccess determines if an agent has write access to an op
func (o *Operation) WriteAccess(gid GoogleID) bool {
	// do not cache -- force reset on uploads
	if err := o.PopulateTeams(); err != nil {
		log.Error(err)
		return false
	}
	if o.ID.IsOwner(gid) {
		return true
	}
	for _, t := range o.Teams {
		if t.Role != opPermRoleWrite {
			continue
		}
		// write teams
		if inteam, _ := gid.AgentInTeam(t.TeamID); inteam {
			return true
		}
	}
	return false
}

// WriteAccess is the lesser form of determining if a gid has write access to an operation, used when the op handle isn't accessible
func (o OperationID) WriteAccess(gid GoogleID) bool {
	if o.IsOwner(gid) {
		return true
	}

	rows, err := db.Query("SELECT teamID FROM opteams WHERE opID = ? AND permission = ?", o, opPermRoleWrite)
	if err != nil && err != sql.ErrNoRows {
		log.Error(err)
		return false
	}

	var tid TeamID
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tid)
		if err != nil {
			log.Error(err)
			continue
		}
		if inteam, _ := gid.AgentInTeam(tid); inteam {
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
		log.Error(err)
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
		err := fmt.Errorf("permission denied: not current owner")
		log.Errorw(err.Error(), "GID", gid, "resource", opID)
		return err
	}

	togid, err := ToGid(to)
	if err != nil {
		log.Error(err)
		return err
	}

	if !togid.Valid() {
		err := fmt.Errorf("unknown user")
		log.Errorw(err.Error(), "to", to)
		return err
	}

	_, err = db.Exec("UPDATE operation SET gid = ? WHERE ID = ?", togid, opID)
	if err != nil {
		log.Error(err)
		return err
	}

	return nil
}

// AssignedOnlyAccess verifies if an agent has AO access to an op
func (o *Operation) AssignedOnlyAccess(gid GoogleID) bool {
	if len(o.Teams) == 0 {
		if err := o.PopulateTeams(); err != nil {
			log.Error(err)
			return false
		}
	}

	for _, t := range o.Teams {
		if t.Role != opPermRoleAssignedOnly {
			continue
		}
		if inteam, _ := gid.AgentInTeam(t.TeamID); inteam {
			return true
		}
	}
	return false
}

// AddPerm adds a new permission to an op
func (o OperationID) AddPerm(gid GoogleID, teamID TeamID, perm string, zone Zone) error {
	if !o.IsOwner(gid) {
		err := fmt.Errorf("permission denied: not current owner of op")
		log.Errorw(err.Error(), "GID", gid, "resource", o)
		return err
	}

	inteam, err := gid.AgentInTeam(teamID)
	if err != nil {
		log.Error(err)
		return err
	}
	if !inteam {
		err := fmt.Errorf("you must be on a team to add it as a permission")
		log.Errorw(err.Error(), "GID", gid, "team", teamID, "resource", o)
		return err
	}

	opp := OpPermRole(perm)
	if !opp.Valid() {
		err := fmt.Errorf("unknown permission type")
		log.Errorw(err.Error(), "GID", gid, "resource", o, "perm", perm)
		return err
	}

	// zone only applies to read access for now
	if opp != opPermRoleRead {
		zone = ZoneAll
	}
	_, err = db.Exec("INSERT INTO opteams VALUES (?,?,?,?)", teamID, o, opp, zone)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// DelPerm removes a permission from an op
func (o OperationID) DelPerm(gid GoogleID, teamID TeamID, perm OpPermRole, zone Zone) error {
	if !o.IsOwner(gid) {
		err := fmt.Errorf("not owner of op")
		log.Errorw(err.Error(), "GID", gid, "resource", o)
		return err
	}

	if perm != opPermRoleRead {
		_, err := db.Exec("DELETE FROM opteams WHERE teamID = ? AND opID = ? AND permission = ? LIMIT 1", teamID, o, perm)
		if err != nil {
			log.Error(err)
			return err
		}
	} else {
		_, err := db.Exec("DELETE FROM opteams WHERE teamID = ? AND opID = ? AND permission = ? AND zone = ? LIMIT 1", teamID, o, perm, zone)
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return nil
}

// Operations returns a slice containing all the OpPermissions which reference this team
func (t TeamID) Operations() ([]OpPermission, error) {
	var perms []OpPermission
	rows, err := db.Query("SELECT opID, permission, zone FROM opteams WHERE teamID = ?", t)
	if err != nil && err != sql.ErrNoRows {
		log.Error(err)
		return perms, err
	}

	var opid, role string
	var zone Zone
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&opid, &role, &zone)
		if err != nil {
			log.Error(err)
			continue
		}
		perms = append(perms, OpPermission{
			OpID:   OperationID(opid),
			TeamID: t,
			Role:   OpPermRole(role),
			Zone:   zone,
		})
	}
	return perms, nil
}

// Teams returns a list of every team with access to this operation
func (o OperationID) Teams() ([]TeamID, error) {
	var teams []TeamID
	rows, err := db.Query("SELECT teamID FROM opteams WHERE opID = ?", o)
	if err != nil && err != sql.ErrNoRows {
		log.Error(err)
		return teams, err
	}

	for rows.Next() {
		var t TeamID
		err := rows.Scan(&t)
		if err != nil {
			log.Error(err)
			continue
		}
		teams = append(teams, t)
	}
	return teams, nil
}
