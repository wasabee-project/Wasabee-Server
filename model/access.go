package model

import (
	"context"
	"errors"

	"github.com/wasabee-project/Wasabee-Server/log"
)

// PopulateTeams loads the permissions from the database into the op data cache
func (o *Operation) PopulateTeams(ctx context.Context) error {
	// do not do duplicate work
	if len(o.Teams) > 0 {
		return nil
	}

	rows, err := db.QueryContext(ctx, "SELECT teamID, permission, zone FROM permissions WHERE opID = ?", o.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var tid, role string
		var zone ZoneID
		if err := rows.Scan(&tid, &role, &zone); err != nil {
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

// ReadAccess determines if an agent has read access to an op.
// If zone limitations are present, it returns those as well.
func (o *Operation) ReadAccess(ctx context.Context, gid GoogleID) (bool, []ZoneID) {
	var zones []ZoneID

	// Owners always have full access to all zones
	if o.ID.IsOwner(ctx, gid) {
		return true, []ZoneID{ZoneAll}
	}

	if err := o.PopulateTeams(ctx); err != nil {
		log.Error(err)
		return false, zones
	}

	permitted := false
	for _, t := range o.Teams {
		switch t.Role {
		case OpPermRoleRead:
			if inteam, _ := gid.AgentInTeam(ctx, t.TeamID); inteam {
				permitted = true
				zones = append(zones, t.Zone)
				if t.Zone == ZoneAll {
					return true, []ZoneID{ZoneAll} // Full access fast-path
				}
			}
		case OpPermRoleWrite:
			// Write implies full Read access
			if inteam, _ := gid.AgentInTeam(ctx, t.TeamID); inteam {
				return true, []ZoneID{ZoneAll}
			}
		}
	}
	return permitted, zones
}

// WriteAccess determines if an agent has write access to an op
func (o *Operation) WriteAccess(ctx context.Context, gid GoogleID) bool {
	if o.ID.IsOwner(ctx, gid) {
		return true
	}

	if err := o.PopulateTeams(ctx); err != nil {
		return false
	}

	for _, t := range o.Teams {
		if t.Role == OpPermRoleWrite {
			if inteam, _ := gid.AgentInTeam(ctx, t.TeamID); inteam {
				return true
			}
		}
	}
	return false
}

// IsOwner returns a bool value determining if the operation is owned by the specified googleID
func (opID OperationID) IsOwner(ctx context.Context, gid GoogleID) bool {
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM operation WHERE ID = ? and gid = ?", opID, gid).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

// Chown changes an operation's owner
func (opID OperationID) Chown(ctx context.Context, gid GoogleID, to string) error {
	if !opID.IsOwner(ctx, gid) {
		return errors.New(ErrNotOpOwner)
	}

	togid, err := ToGid(ctx, to)
	if err != nil {
		return err
	}

	if !togid.Valid(ctx) {
		return errors.New(ErrUnknownUser)
	}

	_, err = db.ExecContext(ctx, "UPDATE operation SET gid = ? WHERE ID = ?", togid, opID)
	return err
}

// AssignedOnlyAccess verifies if an agent has "Assigned Only" access to an op
func (o *Operation) AssignedOnlyAccess(ctx context.Context, gid GoogleID) bool {
	if err := o.PopulateTeams(ctx); err != nil {
		return false
	}

	for _, t := range o.Teams {
		if t.Role == OpPermRoleAssignedOnly {
			if inteam, _ := gid.AgentInTeam(ctx, t.TeamID); inteam {
				return true
			}
		}
	}
	return false
}

// AddPerm adds a new permission to an op
func (opID OperationID) AddPerm(ctx context.Context, gid GoogleID, teamID TeamID, perm string, zone ZoneID) error {
	if !opID.IsOwner(ctx, gid) {
		return errors.New(ErrNotOpOwner)
	}

	inteam, err := gid.AgentInTeam(ctx, teamID)
	if err != nil || !inteam {
		return errors.New(ErrNotOnTeamAddPerm)
	}

	opp := OpPermRole(perm)
	if !opp.Valid() {
		return errors.New(ErrUnknownPermType)
	}

	// Zone only applies to read access
	if opp != OpPermRoleRead {
		zone = ZoneAll
	}

	_, err = db.ExecContext(ctx, "INSERT INTO permissions (teamID, opID, permission, zone) VALUES (?, ?, ?, ?)",
		teamID, opID, opp, zone)
	return err
}

// DelPerm removes a permission from an op
func (opID OperationID) DelPerm(ctx context.Context, gid GoogleID, teamID TeamID, perm OpPermRole, zone ZoneID) error {
	if !opID.IsOwner(ctx, gid) {
		return errors.New(ErrNotOpOwner)
	}

	if perm != OpPermRoleRead {
		_, err := db.ExecContext(ctx, "DELETE FROM permissions WHERE teamID = ? AND opID = ? AND permission = ? LIMIT 1", teamID, opID, perm)
		return err
	}

	_, err := db.ExecContext(ctx, "DELETE FROM permissions WHERE teamID = ? AND opID = ? AND permission = ? AND zone = ? LIMIT 1", teamID, opID, perm, zone)
	return err
}

// Operations returns all OpPermissions referencing this team
func (teamID TeamID) Operations(ctx context.Context) ([]OpPermission, error) {
	rows, err := db.QueryContext(ctx, "SELECT opID, permission, zone FROM permissions WHERE teamID = ?", teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []OpPermission
	for rows.Next() {
		var opid, role string
		var zone ZoneID
		if err := rows.Scan(&opid, &role, &zone); err != nil {
			continue
		}
		perms = append(perms, OpPermission{
			OpID:   OperationID(opid),
			TeamID: teamID,
			Role:   OpPermRole(role),
			Zone:   zone,
		})
	}
	return perms, nil
}

// Teams returns a list of every team with access to this operation
func (opID OperationID) Teams(ctx context.Context) ([]TeamID, error) {
	rows, err := db.QueryContext(ctx, "SELECT DISTINCT teamID FROM permissions WHERE opID = ?", opID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []TeamID
	for rows.Next() {
		var t TeamID
		if err := rows.Scan(&t); err != nil {
			continue
		}
		teams = append(teams, t)
	}
	return teams, nil
}
