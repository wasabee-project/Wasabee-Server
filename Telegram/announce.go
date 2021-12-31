package wtg

import (
	"github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/templates"
)

func sendAnnounce(t messaging.TeamID, a messaging.Announce) error {
	teamID := model.TeamID(t)
	tgchat, err := teamID.TelegramChat()
	if err != nil {
		log.Error(err)
		return err
	}

	// if team not linked to TG chat, silently drop
	if tgchat >= 0 { // chats are negative, agents are positive
		return nil
	}

	text, err := templates.ExecuteLang("announcement", "en", a)
	if err != nil {
		log.Error(err)
		// send it raw
	}
	if text == "" {
		text = a.Text
	}

	msg := tgbotapi.NewMessage(tgchat, text)
	msg.ParseMode = "HTML"

	sendQueue <- msg
	return nil
}
