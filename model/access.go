package model

import (
	"database/sql"
	"fmt"

	"github.com/wasabee-project/Wasabee-Server/log"
)

// PopulateTeams loads the permissions from the database into the op data
func (o *Operation) PopulateTeams() error {
	// do not do duplicate work
	if len(o.Teams) > 0 {
		return nil
	}

	rows, err := db.Query("SELECT teamID, permission, zone FROM permissions WHERE opID = ?", o.ID)
	if err != nil && err != sql.ErrNoRows {
		log.Error(err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var tid, role string
		var zone Zone
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

	if o.IsOwner(gid) {
		permitted = true
		zones = append(zones, ZoneAll)
		return true, zones
	}

	if err := o.PopulateTeams(); err != nil {
		log.Error(err)
		return false, zones
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
	if o.IsOwner(gid) {
		return true
	}

	if err := o.PopulateTeams(); err != nil {
		log.Error(err)
		return false
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

// IsOwner checks to see if the operation is owned a given GoogleID
// result cached so it may be called multiple times
func (o *Operation) IsOwner(gid GoogleID) bool {
	if o.Gid != "" {
		return o.Gid == gid
	}
	if o.ID.IsOwner(gid) {
		o.Gid = gid
		return true
	}
	return false
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
	if err := o.PopulateTeams(); err != nil {
		log.Error(err)
		return false
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
func (opID OperationID) AddPerm(gid GoogleID, teamID TeamID, perm string, zone Zone) error {
	if !opID.IsOwner(gid) {
		err := fmt.Errorf("permission denied: not current owner of op")
		log.Errorw(err.Error(), "GID", gid, "resource", opID)
		return err
	}

	inteam, err := gid.AgentInTeam(teamID)
	if err != nil {
		log.Error(err)
		return err
	}
	if !inteam {
		err := fmt.Errorf("you must be on a team to add it as a permission")
		log.Errorw(err.Error(), "GID", gid, "team", teamID, "resource", opID)
		return err
	}

	opp := OpPermRole(perm)
	if !opp.Valid() {
		err := fmt.Errorf("unknown permission type")
		log.Errorw(err.Error(), "GID", gid, "resource", opID, "perm", perm)
		return err
	}

	// zone only applies to read access for now
	if opp != opPermRoleRead {
		zone = ZoneAll
	}
	if _, err = db.Exec("INSERT INTO permissions VALUES (?,?,?,?)", teamID, opID, opp, zone); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// DelPerm removes a permission from an op
func (opID OperationID) DelPerm(gid GoogleID, teamID TeamID, perm OpPermRole, zone Zone) error {
	if !opID.IsOwner(gid) {
		err := fmt.Errorf("not owner of op")
		log.Errorw(err.Error(), "GID", gid, "resource", opID)
		return err
	}

	if perm != opPermRoleRead {
		if _, err := db.Exec("DELETE FROM permissions WHERE teamID = ? AND opID = ? AND permission = ? LIMIT 1", teamID, opID, perm); err != nil {
			log.Error(err)
			return err
		}
	} else {
		if _, err := db.Exec("DELETE FROM permissions WHERE teamID = ? AND opID = ? AND permission = ? AND zone = ? LIMIT 1", teamID, opID, perm, zone); err != nil {
			log.Error(err)
			return err
		}
	}
	return nil
}

// Operations returns a slice containing all the OpPermissions which reference this team
func (t TeamID) Operations() ([]OpPermission, error) {
	var perms []OpPermission
	rows, err := db.Query("SELECT opID, permission, zone FROM permissions WHERE teamID = ?", t)
	if err != nil && err != sql.ErrNoRows {
		log.Error(err)
		return perms, err
	}
	defer rows.Close()

	for rows.Next() {
		var opid, role string
		var zone Zone
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
func (opID OperationID) Teams() ([]TeamID, error) {
	var teams []TeamID
	rows, err := db.Query("SELECT DISTINCT teamID FROM permissions WHERE opID = ?", opID)
	if err != nil && err != sql.ErrNoRows {
		log.Error(err)
		return teams, err
	}
	defer rows.Close()

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
