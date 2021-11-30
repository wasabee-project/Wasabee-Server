package messaging

import (
	"github.com/wasabee-project/Wasabee-Server/model"
)

func SendMessage(toGID model.GoogleID, message string) (bool, error) {
	return false, nil
}

func CanSendTo(fromGID model.GoogleID, toGID model.GoogleID) bool {
	return false
}
