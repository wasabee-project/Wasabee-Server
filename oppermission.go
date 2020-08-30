package wasabee

/*
import (
	"encoding/json"
	"fmt"
) */

// OpPermission is the form of permission
type OpPermission struct {
	TeamID TeamID     `json:"teamid"`
	Role   opPermRole `json:"role"`
	Zone   Zone       `json:"zone"`
}

type opPermRole string

const (
	opPermRoleRead         opPermRole = "read"
	opPermRoleWrite        opPermRole = "write"
	opPermRoleAssignedOnly opPermRole = "assignedonly"
)

func (perm opPermRole) Valid() bool {
	switch perm {
	case opPermRoleRead, opPermRoleWrite, opPermRoleAssignedOnly:
		return true
	default:
		return false
	}
}

// do we need marshal/unmarshal calls?
