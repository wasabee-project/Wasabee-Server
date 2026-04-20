package wtg

import (
	"context"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/templates"
)

// sendAnnounce broadcasts a team announcement to the linked Telegram chat.
func sendAnnounce(ctx context.Context, t messaging.TeamID, a messaging.Announce) error {
	teamID := model.TeamID(t)

	// Get the chat ID linked to this team
	tgchat, err := teamID.TelegramChat(ctx)
	if err != nil {
		log.Error(err)
		return err
	}

	// Telegram Chat IDs for groups and channels are always negative.
	// If it's 0 (not linked) or positive (somehow an agent ID), we skip.
	if tgchat >= 0 {
		return nil
	}

	// Try to execute the template; fall back to raw text if it fails
	text, err := templates.ExecuteLang("announcement", "en", a)
	if err != nil {
		log.Errorw("template execution failed for announcement", "error", err)
		text = a.Text
	}
	if text == "" {
		text = a.Text
	}

	msg := tgbotapi.NewMessage(tgchat, text)
	msg.ParseMode = "HTML"
	msg.DisableWebPagePreview = false // Announcements might have useful links

	// Hand it off to the rate-limited sender
	sendQueue <- msg
	return nil
}
