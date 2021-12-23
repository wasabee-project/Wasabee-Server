package messaging

import (
	"fmt"

	"github.com/wasabee-project/Wasabee-Server/log"
)

// model includes this package, so it can't include model, to enforce type-consistency, we re-define some types from model and rely on the callers to cast them
type GoogleID string
type TeamID string
type TaskID string
type OperationID string

type Target struct {
	Name   string
	ID     string
	Lat    string
	Lng    string
	Type   string
	Sender string
}

type Bus struct {
	SendMessage          func(GoogleID, string) (bool, error)              // send a message to an individual agent
	SendTarget           func(GoogleID, Target) error                      // send a formatted target to an individual agent
	CanSendTo            func(fromGID GoogleID, toGID GoogleID) bool       // determine if one agent can send to another
	SendAnnounce         func(TeamID, string) error                        // send a messaage to a team
	AddToRemote          func(GoogleID, TeamID) error                      // add an agent to a services chat/community/team/channel/whatever
	RemoveFromRemote     func(GoogleID, TeamID) error                      // remove an agent from a service's X
	SendAssignment       func(GoogleID, TaskID, OperationID, string) error // Send a formatted assignment to an individual agent
	AgentDeleteOperation func(GoogleID, OperationID) error                 // instruct a single agent to delete an operation
	DeleteOperation      func(OperationID) error                           // instruct EVERYONE to delete an operation
}

var busses map[string]Bus

func init() {
	busses = make(map[string]Bus)
}

func SendTarget(toGID GoogleID, target Target) error {
	if target.Name == "" {
		err := fmt.Errorf("portal not set")
		log.Warnw(err.Error(), "GID", toGID)
		return err
	}

	if target.Lat == "" || target.Lng == "" {
		err := fmt.Errorf("lat/lng not set")
		log.Warnw(err.Error(), "GID", toGID)
		return err
	}

	for _, bus := range busses {
		if bus.SendTarget != nil {
			if err := bus.SendTarget(toGID, target); err != nil {
				log.Error(err)
			}
		}
	}

	return nil
}

func SendMessage(toGID GoogleID, message string) (bool, error) {
	var sent bool

	for name, bus := range busses {
		if bus.SendMessage != nil {
			success, err := bus.SendMessage(toGID, message)
			if err != nil {
				log.Error(err)
			}
			if success {
				sent = true
				log.Infow("message sent", "toGID", toGID, "bus", name, "message", message)
			}
		}
	}
	return sent, nil
}

func SendAnnounce(teamID TeamID, message string) error {
	for _, bus := range busses {
		if bus.SendAnnounce != nil {
			if err := bus.SendAnnounce(teamID, message); err != nil {
				log.Error(err)
			}
		}
	}
	return nil
}

func RegisterMessageBus(busname string, b Bus) {
	busses[busname] = b
}

func RemoveMessageBus(busname string) {
	delete(busses, busname)
}

func AddToRemote(gid GoogleID, teamID TeamID) error {
	for _, bus := range busses {
		if bus.AddToRemote != nil {
			if err := bus.AddToRemote(gid, teamID); err != nil {
				log.Error(err)
			}
		}
	}
	return nil
}

func RemoveFromRemote(gid GoogleID, teamID TeamID) error {
	for _, bus := range busses {
		if bus.RemoveFromRemote != nil {
			if err := bus.RemoveFromRemote(gid, teamID); err != nil {
				log.Error(err)
			}
		}
	}
	return nil
}

func SendAssignment(gid GoogleID, taskID TaskID, opID OperationID, status string) error {
	for _, bus := range busses {
		if bus.SendAssignment != nil {
			if err := bus.SendAssignment(gid, taskID, opID, status); err != nil {
				log.Error(err)
			}
		}
	}
	return nil
}

func AgentDeleteOperation(gid GoogleID, opID OperationID) error {
	for _, bus := range busses {
		if bus.AgentDeleteOperation != nil {
			if err := bus.AgentDeleteOperation(gid, opID); err != nil {
				log.Error(err)
			}
		}
	}
	return nil
}

func DeleteOperation(opID OperationID) error {
	for _, bus := range busses {
		if bus.DeleteOperation != nil {
			if err := bus.DeleteOperation(opID); err != nil {
				log.Error(err)
			}
		}
	}
	return nil
}
