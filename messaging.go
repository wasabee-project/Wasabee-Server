package wasabi

import (
	"fmt"
)

type messagingConfig struct {
	inited       bool
	senders      map[string]func(gid GoogleID, message string) (bool, error)
}

var mc messagingConfig

// SendMessage sends a message to the available message destinations for agent specified by "gid"
// currently only Telegram is supported, but more can be added
func (gid GoogleID) SendMessage(message string) (bool, error) {
	// determine which messaging protocols are enabled for gid
	// pick optimal
	bus := "Telegram"

	// XXX loop through valid, trying until one works
	ok, err := gid.SendMessageVia(message, bus)
	if err != nil {
		Log.Notice("unable to send message")
		return false, err
	}
	if !ok {
		err = fmt.Errorf("unable to send message")
		return false, err
	}
	return true, nil
}

// SendMessageVia sends a message to the destination on the specified bus
func (gid GoogleID) SendMessageVia(message, bus string) (bool, error) {
	_, ok := mc.senders[bus]
	if !ok {
		err := fmt.Errorf("no such messaging bus: [%s]", bus)
		return false, err
	}

	ok, err := mc.senders[bus](gid, message)
	if err != nil {
		Log.Notice(err)
		return false, err
	}
	return ok, nil
}

// SendAnnounce sends a message to everyone on the team, determining what is the best route per agent
func (teamID TeamID) SendAnnounce(message string) error {
	// for each agent on the team
	// determine which messaging protocols are enabled for gid
	// pick optimal

	// ok, err := SendMessage(gid, message)
	return nil
}

// RegisterMessageBus registers a function used to send messages by various protocols
func RegisterMessageBus(name string, f func(GoogleID, string) (bool, error)) error {
	mc.senders[name] = f
	return nil
}

// called at server start to init the configuration
func init() {
	mc.senders = make(map[string]func(GoogleID, string) (bool, error))
	mc.inited = true
}
