package wasabee

import (
	"fmt"
	"strconv"
)

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

	ps.c <- PSCommand{
		Command: "request",
		Param:   gid.String(),
	}
}

// Push an Agent's location to PubSub
func (gid GoogleID) PSLocation(lat, lon string) {
	if !ps.running {
		return
	}

	flat, err := strconv.ParseFloat(lat, 64)
	if err != nil {
		Log.Error(err)
		flat = float64(0)
	}
	flon, err := strconv.ParseFloat(lon, 64)
	if err != nil {
		Log.Error(err)
		flon = float64(0)
	}
	ll := fmt.Sprintf("%s,%s", strconv.FormatFloat(flon, 'f', 7, 64), strconv.FormatFloat(flat, 'f', 7, 64))

	ps.c <- PSCommand{
		Command: "location",
		Param:   gid.String(),
		Data:    ll,
	}
}

// Pushes the agent name from intel to PubSub
func (gid GoogleID) PSIntelData(name, faction string) {
	if !ps.running {
		return
	}

	data := fmt.Sprintf("%s,%s", name, faction)

	ps.c <- PSCommand{
		Command: "inteldata",
		Param:   gid.String(),
		Data:    data,
	}
}
