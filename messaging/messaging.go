package messaging

import (
	"fmt"

	"github.com/wasabee-project/Wasabee-Server/log"
)

// wrapper
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
	log.Debugw("send target", "gid", toGID, "target", target)

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

	for name, bus := range busses {
		if bus.SendTarget != nil {
			err := bus.SendTarget(toGID, target)
			if err != nil {
				log.Error(err)
			}
			log.Infow("target sent", "toGID", toGID, "bus", name, "message", target)
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

func CanSendTo(fromGID GoogleID, toGID GoogleID) bool {
	log.Error("cansendto called but not written")
	return false
}

func SendAnnounce(teamID TeamID, message string) error {
	for name, bus := range busses {
		if bus.SendAnnounce != nil {
			err := bus.SendAnnounce(teamID, message)
			if err != nil {
				log.Error(err)
			}
			log.Infow("announcement sent", "bus", name, "teamID", teamID)
		}
	}
	return nil
}

func RegisterMessageBus(busname string, b Bus) {
	busses[busname] = b
}

func AddToRemote(gid GoogleID, teamID TeamID) error {
	for name, bus := range busses {
		if bus.AddToRemote != nil {
			err := bus.AddToRemote(gid, teamID)
			if err != nil {
				log.Error(err)
			}
			log.Infow("added to remote", "gid", gid, "bus", name, "teamID", teamID)
		}
	}
	return nil
}

func RemoveFromRemote(gid GoogleID, teamID TeamID) error {
	for name, bus := range busses {
		if bus.RemoveFromRemote != nil {
			err := bus.RemoveFromRemote(gid, teamID)
			if err != nil {
				log.Error(err)
			}
			log.Infow("removed from remote", "gid", gid, "bus", name, "teamID", teamID)
		}
	}
	return nil
}

func SendAssignment(gid GoogleID, taskID TaskID, opID OperationID, status string) error {
	for name, bus := range busses {
		if bus.SendAssignment != nil {
			err := bus.SendAssignment(gid, taskID, opID, status)
			if err != nil {
				log.Error(err)
			}
			log.Infow("removed from remote", "gid", gid, "taskID", taskID, "bus", name)
		}
	}
	return nil
}

func AgentDeleteOperation(gid GoogleID, opID OperationID) error {
	for name, bus := range busses {
		if bus.AgentDeleteOperation != nil {
			err := bus.AgentDeleteOperation(gid, opID)
			if err != nil {
				log.Error(err)
			}
			log.Infow("removed from remote", "gid", gid, "resource", opID, "bus", name)
		}
	}
	return nil
}

func DeleteOperation(opID OperationID) error {
	for _, bus := range busses {
		if bus.DeleteOperation != nil {
			err := bus.DeleteOperation(opID)
			if err != nil {
				log.Error(err)
			}
		}
	}
	return nil
}
