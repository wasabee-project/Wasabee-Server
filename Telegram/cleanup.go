package wtg

import (
	"context"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

const agentHasNotStartedBot = "Bad Request: chat not found"

func cleanup(ctx context.Context) error {
	tgs, err := model.GetAllTelegramIDs()
	if err != nil {
		log.Error(err)
		return err
	}

	for _, v := range tgs {
		select {
		case <-ctx.Done():
			log.Info("Shutting down Telegram cleanup")
			return nil
		default:
			cic := tgbotapi.ChatInfoConfig{}
			cic.ChatID = int64(v)
			chat, err := bot.GetChat(cic)

			if err != nil && err.Error() != agentHasNotStartedBot {
				// other errors are hard fails
				log.Errorw(err.Error(), "chatID", v)

				gid, err := v.Gid()
				if err != nil {
					log.Error(err)
					continue
				}
				_ = gid.RemoveTelegramID()
				continue
			}

			if chat.Type != "private" {
				log.Debugw("not private chat", "chat", chat, "chatID", v)
				// v.Delete()
				// v.UnverifyAgent()
				continue
			}

			if chat.UserName == "" {
				log.Debugw("no telegram username", "TelegramID", v, "chat", chat)
				continue
			}

			// log.Debugw("updating username", "name", chat.UserName, "chatID", v)
			_ = v.SetName(chat.UserName)
		}
	}
	return nil
}
