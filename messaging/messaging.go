package messaging

import (
	"context"
	"fmt"

	"github.com/wasabee-project/Wasabee-Server/log"
)

// types remain the same (GoogleID, TeamID, etc.)
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

type Announce struct {
	Text   string
	Sender GoogleID
	OpID   OperationID
	TeamID TeamID
}

// Bus is now context-aware
type Bus struct {
	SendMessage          func(context.Context, GoogleID, string) (bool, error)
	SendTarget           func(context.Context, GoogleID, Target) error
	CanSendTo            func(fromGID GoogleID, toGID GoogleID) bool // pure logic, no context needed usually
	SendAnnounce         func(context.Context, TeamID, Announce) error
	AddToRemote          func(context.Context, GoogleID, TeamID) error
	RemoveFromRemote     func(context.Context, GoogleID, TeamID) error
	SendAssignment       func(context.Context, GoogleID, TaskID, OperationID, string) error
	AgentDeleteOperation func(context.Context, GoogleID, OperationID) error
	DeleteOperation      func(context.Context, OperationID) error
}

var busses map[string]Bus

func init() {
	busses = make(map[string]Bus)
}

func SendTarget(ctx context.Context, toGID GoogleID, target Target) error {
	if target.Name == "" {
		return fmt.Errorf("portal not set")
	}
	if target.Lat == "" || target.Lng == "" {
		return fmt.Errorf("lat/lng not set")
	}

	for _, bus := range busses {
		if bus.SendTarget != nil {
			if err := bus.SendTarget(ctx, toGID, target); err != nil {
				log.Error(err)
			}
		}
	}
	return nil
}

func SendMessage(ctx context.Context, toGID GoogleID, message string) (bool, error) {
	var sent bool
	for name, bus := range busses {
		if bus.SendMessage != nil {
			success, err := bus.SendMessage(ctx, toGID, message)
			if err != nil {
				log.Error(err)
			}
			if success {
				sent = true
				log.Infow("message sent", "toGID", toGID, "bus", name)
			}
		}
	}
	return sent, nil
}

func SendAnnounce(ctx context.Context, teamID TeamID, a Announce) {
	for _, bus := range busses {
		if bus.SendAnnounce != nil {
			if err := bus.SendAnnounce(ctx, teamID, a); err != nil {
				log.Error(err)
			}
		}
	}
}

func RegisterMessageBus(busname string, b Bus) {
	busses[busname] = b
}

func AddToRemote(ctx context.Context, gid GoogleID, teamID TeamID) {
	for _, bus := range busses {
		if bus.AddToRemote != nil {
			if err := bus.AddToRemote(ctx, gid, teamID); err != nil {
				log.Error(err)
			}
		}
	}
}

func RemoveFromRemote(ctx context.Context, gid GoogleID, teamID TeamID) {
	for _, bus := range busses {
		if bus.RemoveFromRemote != nil {
			if err := bus.RemoveFromRemote(ctx, gid, teamID); err != nil {
				log.Error(err)
			}
		}
	}
}

func SendAssignment(ctx context.Context, gid GoogleID, taskID TaskID, opID OperationID, status string) {
	for _, bus := range busses {
		if bus.SendAssignment != nil {
			if err := bus.SendAssignment(ctx, gid, taskID, opID, status); err != nil {
				log.Error(err)
			}
		}
	}
}

func AgentDeleteOperation(ctx context.Context, gid GoogleID, opID OperationID) {
	for _, bus := range busses {
		if bus.AgentDeleteOperation != nil {
			if err := bus.AgentDeleteOperation(ctx, gid, opID); err != nil {
				log.Error(err)
			}
		}
	}
}

func DeleteOperation(ctx context.Context, opID OperationID) {
	for _, bus := range busses {
		if bus.DeleteOperation != nil {
			if err := bus.DeleteOperation(ctx, opID); err != nil {
				log.Error(err)
			}
		}
	}
}
