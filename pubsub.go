package wasabee

var ps struct {
	running bool
	c       chan PSCommand
}

// PSCommand is the command passed over the channel
// Currently there is online command: request agent data from the responders
// Potential future options: push agent location, sync op, sync team, etc... etc...
// as we determine what is desired/needed
type PSCommand struct {
	Command string
	Param   string
	Data    string
}

// PubSubInit creates the channel used to pass messages to the PubSub subsystem
func PubSubInit() <-chan PSCommand {
	out := make(chan PSCommand)

	ps.c = out
	ps.running = true
	return out
}

// PubSubClose shuts down the channel when done
func PubSubClose() {
	if ps.running {
		Log.Infow("shutdown", "subsystem", "PubSub", "message", "shutting down PubSub")
		ps.running = false
		close(ps.c)
	}
}

// PSRequest is used to request user data from the Pub/Sub subsystem
func (gid GoogleID) PSRequest() {
	if !ps.running {
		return
	}

	Log.Debugw("PubSub request", "GID", gid.String())
	ps.c <- PSCommand{
		Command: "request",
		Param:   gid.String(),
	}
}
