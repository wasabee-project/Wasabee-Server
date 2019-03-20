package PhDevBin

import (
	"errors"
)

type messagingConfig struct {
	inited  bool
	senders map[string]func(string, string) (bool, error)
}

var mc messagingConfig

func SendMessage(gid, message string) (bool, error) {
	// determine which messaging protocols are enabled for gid
	// pick optimal
	bus := "Telegram"

	// XXX loop through valid, trying until one works
	ok, err := SendMessageVia(gid, message, bus)
	if err != nil {
		Log.Notice("Unable to send message")
		return false, err
	}
	if ok == false {
		err = errors.New("Unable to send message")
		return false, err
	}
	return true, nil
}

func SendMessageVia(gid, message, bus string) (bool, error) {
	_, ok := mc.senders[bus]
	if ok == false {
		err := errors.New("No such bus")
		return false, err
	}

	ok, err := mc.senders[bus](gid, message)
	if err != nil {
		Log.Error(err)
		return false, err
	}
	return ok, nil
}

func SendAnnounce(teamID, message string) error {
	// for each user on the team
	// determine which messaging protocols are enabled for gid
	// pick optimal

	// ok, err := SendMessage(gid, message)
	return nil
}

func PhDevMessagingRegister(name string, f func(string, string) (bool, error)) error {
	mc.senders[name] = f
	return nil
}

func init() {
	mc.senders = make(map[string]func(string, string) (bool, error))
	mc.inited = true
}
