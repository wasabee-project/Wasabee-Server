package messaging

import (
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

func SendMessage(toGID model.GoogleID, message string) (bool, error) {
	return false, nil
}

func CanSendTo(fromGID model.GoogleID, toGID model.GoogleID) bool {
	return false
}

func SendAnnounce(teamID model.TeamID, message string) (bool, error) {
	log.Error("send announce called but not written")
	return false, nil
}

func RegisterMessageBus(busname string, send func(model.GoogleID, string) (bool, error)) {
	log.Error("RegisterMessageBus called but not written")
}

func RegisterGroupCalls(busname string, addtochat func(model.GoogleID, string) (bool, error), removefromchat func(model.GoogleID, string) (bool, error)) {
	log.Error("RegisterGroupCalls called but not written")
}
