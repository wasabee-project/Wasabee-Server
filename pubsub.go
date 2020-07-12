package wasabee

var ps struct {
	running bool
	c       chan string
}

// PubSubInit creates the channel used to pass messages to the PubSub subsystem
func PubSubInit() <-chan string {
	out := make(chan string, 3)

	ps.c = out
	ps.running = true
	return out
}

// PubSubClose shuts down the channel when done
func PubSubClose() {
	if ps.running {
		Log.Debug("shutting down PubSub")
		ps.running = false
		close(ps.c)
	}
}
