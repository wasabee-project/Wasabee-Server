package wasabee

import (
	"fmt"
)

type messagingConfig struct {
	inited  bool
	senders map[string]func(gid GoogleID, message string) (bool, error)
	// busses map[string]MessageBus
}

/*
type MessageBus interface {
	SendMessage(gid GoogleID, message string) (bool, error)
}
*/

var mc messagingConfig

// SendMessage sends a message to the available message destinations for agent specified by "gid"
// currently only Telegram is supported, but more can be added
func (gid GoogleID) SendMessage(message string) (bool, error) {
	// Always send via Firebase
	gid.FirebaseGenericMessage(message)

	// determine which messaging protocols are enabled for gid
	// pick optimal
	bus := "Telegram"

	_, err := db.Exec("INSERT INTO messagelog (gid, message) VALUES (?, ?)", gid, message)
	if err != nil {
		return false, err
	}

	// XXX loop through valid, trying until one works
	ok, err := gid.SendMessageVia(message, bus)
	if err != nil {
		Log.Infow("error sending message", "GID", gid, "bus", bus, "error", err.Error(), "message", message)
		return false, err
	}
	if !ok {
		Log.Infow("unable to send message", "GID", gid, "bus", bus, "message", message)
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
		Log.Error(err)
		return false, err
	}
	return ok, nil
}

// CanSendTo checks to see if a message is permitted to be sent between these users
func (gid GoogleID) CanSendTo(to GoogleID) bool {
	// sender must own at least one team on which the receiver is enabled
	var count int
	if err := db.QueryRow("SELECT COUNT(x.gid) FROM agentteams=x, team=t WHERE t.teamID = x.teamID AND t.owner = ? AND x.state != 'Off' AND x.gid = ?", gid, to).Scan(&count); err != nil {
		Log.Error(err)
		return false
	}
	if count < 1 {
		return false
	}
	return true
}

// SendAnnounce sends a message to everyone on the team, determining what is the best route per agent
func (teamID TeamID) SendAnnounce(sender GoogleID, message string) error {
	if owns, _ := sender.OwnsTeam(teamID); !owns {
		err := fmt.Errorf("permission denied: %s sending to team %s", sender, teamID)
		Log.Error(err)
		return err
	}

	rows, err := db.Query("SELECT gid FROM agentteams WHERE teamID = ? AND state != 'Off'", teamID)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rows.Close()

	var gid GoogleID
	for rows.Next() {
		err := rows.Scan(&gid)
		if err != nil {
			Log.Error(err)
			return err
		}
		_, err = gid.SendMessage(message)
		if err != nil {
			Log.Error(err)
			return err
		}
	}

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
