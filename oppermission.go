package wasabee

/*
import (
	"encoding/json"
	"fmt"
) */

// OpPermission is the form of permission
type OpPermission struct {
	TeamID TeamID     `json:"teamid"`
	Role   OpPermRole `json:"role"`
	Zone   Zone       `json:"zone"`
}

type OpPermRole string

const (
	opPermRoleRead         OpPermRole = "read"
	opPermRoleWrite        OpPermRole = "write"
	opPermRoleAssignedOnly OpPermRole = "assignedonly"
)

func (perm OpPermRole) Valid() bool {
	switch perm {
	case opPermRoleRead, opPermRoleWrite, opPermRoleAssignedOnly:
		return true
	default:
		return false
	}
}

// do we need marshal/unmarshal calls?
