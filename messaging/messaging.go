package messaging

import (
	"fmt"

	"github.com/wasabee-project/Wasabee-Server"
	"github.com/wasabee-project/Wasabee-Server/log"
)

type Target struct {
	Name   string
	ID     string
	Lat    string
	Lng    string
	Type   string
	Sender string
}

type Bus struct {
	SendMessage          func(w.GoogleID, string) (bool, error)                  // send a message to an individual agent
	SendTarget           func(w.GoogleID, Target) error                          // send a formatted target to an individual agent
	CanSendTo            func(fromGID w.GoogleID, toGID w.GoogleID) bool         // determine if one agent can send to another
	SendAnnounce         func(w.TeamID, string) error                            // send a messaage to a team
	AddToRemote          func(w.GoogleID, w.TeamID) error                        // add an agent to a services chat/community/team/channel/whatever
	RemoveFromRemote     func(w.GoogleID, w.TeamID) error                        // remove an agent from a service's X
	SendAssignment       func(w.GoogleID, w.TaskID, w.OperationID, string) error // Send a formatted assignment to an individual agent
	AgentDeleteOperation func(w.GoogleID, w.OperationID) error                   // instruct a single agent to delete an operation
	DeleteOperation      func(w.OperationID) error                               // instruct EVERYONE to delete an operation
}

var busses map[string]Bus

func init() {
	busses = make(map[string]Bus)
}

func SendTarget(toGID w.GoogleID, target Target) error {
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
			log.Info("target sent", "toGID", toGID, "bus", name, "message", target)
		}
	}

	return nil
}

func SendMessage(toGID w.GoogleID, message string) (bool, error) {
	var sent bool

	for name, bus := range busses {
		if bus.SendMessage != nil {
			success, err := bus.SendMessage(toGID, message)
			if err != nil {
				log.Error(err)
			}
			if success {
				sent = true
				log.Info("message sent", "toGID", toGID, "bus", name, "message", message)
			}
		}
	}
	return sent, nil
}

func CanSendTo(fromGID w.GoogleID, toGID w.GoogleID) bool {
	log.Error("cansendto called but not written")
	return false
}

func SendAnnounce(teamID w.TeamID, message string) error {
	for name, bus := range busses {
		if bus.SendAnnounce != nil {
			err := bus.SendAnnounce(teamID, message)
			if err != nil {
				log.Error(err)
			}
			log.Info("announcement sent", "bus", name, "teamID", teamID)
		}
	}
	return nil
}

func RegisterMessageBus(busname string, b Bus) {
	busses[busname] = b
}

func AddToRemote(gid w.GoogleID, teamID w.TeamID) error {
	for name, bus := range busses {
		if bus.AddToRemote != nil {
			err := bus.AddToRemote(gid, teamID)
			if err != nil {
				log.Error(err)
			}
			log.Info("added to remote", "gid", gid, "bus", name, "teamID", teamID)
		}
	}
	return nil
}

func RemoveFromRemote(gid w.GoogleID, teamID w.TeamID) error {
	for name, bus := range busses {
		if bus.RemoveFromRemote != nil {
			err := bus.RemoveFromRemote(gid, teamID)
			if err != nil {
				log.Error(err)
			}
			log.Info("removed from remote", "gid", gid, "bus", name, "teamID", teamID)
		}
	}
	return nil
}

func SendAssignment(gid w.GoogleID, taskID w.TaskID, opID w.OperationID, status string) error {
	for name, bus := range busses {
		if bus.SendAssignment != nil {
			err := bus.SendAssignment(gid, taskID, opID, status)
			if err != nil {
				log.Error(err)
			}
			log.Info("removed from remote", "gid", gid, "taskID", taskID, "bus", name)
		}
	}
	return nil
}

func AgentDeleteOperation(gid w.GoogleID, opID w.OperationID) error {
	for name, bus := range busses {
		if bus.AgentDeleteOperation != nil {
			err := bus.AgentDeleteOperation(gid, opID)
			if err != nil {
				log.Error(err)
			}
			log.Info("removed from remote", "gid", gid, "resource", opID, "bus", name)
		}
	}
	return nil
}

func DeleteOperation(opID w.OperationID) error {
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
