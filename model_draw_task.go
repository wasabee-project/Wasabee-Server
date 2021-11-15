package wasabee

// TaskID wrapper to ensure type safety
type TaskID string

// String returns the string version of a TaskID
func (t TaskID) String() string {
	return string(t)
}

type futureTaskID interface {
	Zone(*Operation, Zone) (string, error)
	Delta(*Operation, int) (string, error)
	// String() string
	// isAsignee(*Operation, GoogleID) (bool, error)
	// Acknowledge
	// Complete
	// Incomplete
	// Reject
	// Order ?
	// SetZone
}
