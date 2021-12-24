package wps

import (
	"fmt"
	"strconv"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

var ps struct {
	running bool
	c       chan command
}

// command is the command passed over the channel
// Currently there is online command: request agent data from the responders
// Potential future options: push agent location, sync op, sync team, etc... etc...
// as we determine what is desired/needed
type command struct {
	Command string
	Param   string
	Data    string
}

// startup creates the channel used to pass messages to the PubSub subsystem
func startup() <-chan command {
	out := make(chan command)

	ps.c = out
	ps.running = true
	return out
}

// Stop shuts down the channel when done
func Stop() {
	if ps.running {
		log.Infow("shutdown", "subsystem", "PubSub", "message", "shutting down PubSub")
		ps.running = false
		close(ps.c)
	}
}

// Request is used to request user data from the Pub/Sub subsystem -- not needed now that we have plenty of Rocks keys
func Request(gid model.GoogleID) {
	if !ps.running {
		return
	}

	ps.c <- command{
		Command: "request",
		Param:   gid.String(),
	}
}

// Location pushes an agent's location to PubSub
func Location(gid model.GoogleID, lat, lon string) {
	if !ps.running {
		return
	}

	flat, err := strconv.ParseFloat(lat, 64)
	if err != nil {
		log.Error(err)
		flat = float64(0)
	}
	flon, err := strconv.ParseFloat(lon, 64)
	if err != nil {
		log.Error(err)
		flon = float64(0)
	}
	ll := fmt.Sprintf("%s,%s", strconv.FormatFloat(flon, 'f', 7, 64), strconv.FormatFloat(flat, 'f', 7, 64))

	ps.c <- command{
		Command: "location",
		Param:   gid.String(),
		Data:    ll,
	}
}

// IntelData pushes the agent name from intel to PubSub
func IntelData(gid model.GoogleID, name, faction string) {
	if !ps.running {
		return
	}

	data := fmt.Sprintf("%s,%s", name, faction)

	ps.c <- command{
		Command: "inteldata",
		Param:   gid.String(),
		Data:    data,
	}
}
