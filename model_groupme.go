package wasabee

var gmrunning bool

// GMSetBot is called from the Telegram bot startup to let other services know it is running
func GMSetBot() {
	gmrunning = true
}

// GMRunning is used by templates to determine if they should display telegram info
func GMRunning() (bool, error) {
	return gmrunning, nil
}
