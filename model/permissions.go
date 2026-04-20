package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/wasabee-project/Wasabee-Server/log"
)

// OpPermission is the form of permission
type OpPermission struct {
	OpID   OperationID `json:"opid"`
	TeamID TeamID      `json:"teamid"`
	Role   OpPermRole  `json:"role"`
	Zone   ZoneID      `json:"zone"`
}

// OpPermRole is just a convenience class for the permission string
type OpPermRole string

const (
	OpPermRoleRead         OpPermRole = "read"
	OpPermRoleWrite        OpPermRole = "write"
	OpPermRoleAssignedOnly OpPermRole = "assignedonly"
)

// Valid checks to make sure the OpPermRole is one of the valid options
func (perm OpPermRole) Valid() bool {
	switch perm {
	case OpPermRoleRead, OpPermRoleWrite, OpPermRoleAssignedOnly:
		return true
	default:
		return false
	}
}

func (perm OpPermRole) String() string {
	return string(perm)
}

// AddPermission adds a team to an operation
func (opID OperationID) AddPermission(ctx context.Context, teamID TeamID, role OpPermRole, zone ZoneID) error {
	if !role.Valid() {
		return fmt.Errorf("invalid role: %s", role)
	}

	_, err := db.ExecContext(ctx, "INSERT INTO permissions (opID, teamID, permission, zone) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE permission = ?, zone = ?",
		opID, teamID, role, zone, role, zone)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// DelPermission removes a team from an operation
func (opID OperationID) DelPermission(ctx context.Context, teamID TeamID, role OpPermRole) error {
	_, err := db.ExecContext(ctx, "DELETE FROM permissions WHERE opID = ? AND teamID = ? AND permission = ?", opID, teamID, role)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// GetPermissions returns all permissions for an operation
func (opID OperationID) GetPermissions(ctx context.Context) ([]OpPermission, error) {
	var perms []OpPermission
	rows, err := db.QueryContext(ctx, "SELECT teamID, permission, zone FROM permissions WHERE opID = ?", opID)
	if err != nil {
		if err == sql.ErrNoRows {
			return perms, nil
		}
		log.Error(err)
		return perms, err
	}
	defer rows.Close()

	for rows.Next() {
		var p OpPermission
		p.OpID = opID
		if err := rows.Scan(&p.TeamID, &p.Role, &p.Zone); err != nil {
			log.Error(err)
			continue
		}
		perms = append(perms, p)
	}
	return perms, nil
}
