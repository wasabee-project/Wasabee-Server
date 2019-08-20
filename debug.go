package wasabee

import "time"

var enableLongTimeouts bool

// SetupDebug - setup debugging settings
func SetupDebug(longTimeouts bool) {
	enableLongTimeouts = longTimeouts
}

// GetTimeout returns the passed in timeout, or 1 hour if longTimeouts is set
func GetTimeout(defaultTimeout time.Duration) time.Duration {
	if(enableLongTimeouts) {
		return 1 * time.Hour
	}
	return defaultTimeout
}
