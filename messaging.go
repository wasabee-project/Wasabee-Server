package wasabee

import (
	"fmt"
)

type messagingConfig struct {
	inited bool
	// function to send a message to a GID
	senders map[string]func(gid GoogleID, message string) (bool, error)
	// function to join a channel for a GID
	joiners map[string]func(gid GoogleID, groupID string) (bool, error)
	// function to leave a channel for a GID
	leavers map[string]func(gid GoogleID, groupID string) (bool, error)
}

var mc messagingConfig

// SendMessage sends a message to the available message destinations for agent specified by "gid"
// currently only Telegram is supported, but more can be added
func (gid GoogleID) SendMessage(message string) (bool, error) {
	// determine which messaging protocols are enabled for gid
	// pick optimal
	bus := "Telegram"

	_, err := db.Exec("INSERT INTO messagelog (gid, message) VALUES (?, ?)", gid, message)
	if err != nil {
		return false, err
	}

	// XXX loop through valid, trying until one works
	ok, err := gid.sendMessageVia(message, bus)
	if err != nil {
		Log.Errorw("error sending message", "GID", gid, "bus", bus, "error", err.Error(), "message", message)
		return false, err
	}
	if !ok {
		Log.Infow("unable to send message", "GID", gid, "bus", bus, "message", message)
		return false, err
	}

	return true, nil
}

// SendMessageVia sends a message to the destination on the specified bus
func (gid GoogleID) sendMessageVia(message, bus string) (bool, error) {
	_, ok := mc.senders[bus]
	if !ok {
		err := fmt.Errorf("no such messaging bus: [%s]", bus)
		return false, err
	}
	return mc.senders[bus](gid, message)
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
func RegisterMessageBus(name string, f func(GoogleID, string) (bool, error)) {
	mc.senders[name] = f
}

// called at server start to init the configuration
func init() {
	mc.senders = make(map[string]func(GoogleID, string) (bool, error))
	mc.joiners = make(map[string]func(GoogleID, string) (bool, error))
	mc.leavers = make(map[string]func(GoogleID, string) (bool, error))
	mc.inited = true
}

// RegisterGroupCalls registers the functions to join and leave a channel
func RegisterGroupCalls(name string, join func(GoogleID, string) (bool, error), leave func(GoogleID, string) (bool, error)) {
	mc.joiners[name] = join
	mc.leavers[name] = leave
}

// joinChannels is called when a user is added to a team to add them to the proper messaging service channels
func (gid GoogleID) joinChannels(t TeamID) {
	for service, joiner := range mc.joiners {
		// Log.Debugw("calling joiner", "service", service, "resource", t, "GID", gid)
		_, err := joiner(gid, string(t))
		if err != nil {
			Log.Errorw(err.Error(), "service", service, "resource", t, "GID", gid)
			continue
		}
	}
}

// leaveChannels is called when a user is removed from a team to remove them from the associated messaging service channels
func (gid GoogleID) leaveChannels(t TeamID) {
	for service, leaver := range mc.leavers {
		// Log.Debugw("calling leaver", "service", service, "resource", t, "GID", gid)
		_, err := leaver(gid, string(t))
		if err != nil {
			Log.Errorw(err.Error(), "service", service, "resource", t, "GID", gid)
			continue
		}
	}
}
