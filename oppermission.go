package wasabee

// OpPermission is the form of permission
type OpPermission struct {
	TeamID TeamID     `json:"teamid"`
	Role   OpPermRole `json:"role"`
	Zone   Zone       `json:"zone"`
}

// OpPermRole is just a convenience class for the permission string
type OpPermRole string

const (
	opPermRoleRead         OpPermRole = "read"
	opPermRoleWrite        OpPermRole = "write"
	opPermRoleAssignedOnly OpPermRole = "assignedonly"
)

// Valid checks to make sure the OpPermRole is one of the valid options
func (perm OpPermRole) Valid() bool {
	switch perm {
	case opPermRoleRead, opPermRoleWrite, opPermRoleAssignedOnly:
		return true
	default:
		return false
	}
}
