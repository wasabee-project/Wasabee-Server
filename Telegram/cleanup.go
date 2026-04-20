package wtg

import (
	"context"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

const agentHasNotStartedBot = "Bad Request: chat not found"

// cleanup reconciles our local database of Telegram IDs/usernames with the actual state on Telegram's servers.
func cleanup(ctx context.Context) error {
	log.Info("Starting Telegram state cleanup")

	tgs, err := model.GetAllTelegramIDs(ctx)
	if err != nil {
		log.Error(err)
		return err
	}

	for _, tgid := range tgs {
		select {
		case <-ctx.Done():
			log.Info("Shutting down Telegram cleanup")
			return nil
		default:
			// Rate limit protection: Telegram is sensitive about GetChat on loop.
			// 100ms delay keeps us well under the 30 requests/sec burst limit.
			time.Sleep(100 * time.Millisecond)

			cic := tgbotapi.ChatInfoConfig{
				ChatConfig: tgbotapi.ChatConfig{
					ChatID: int64(tgid),
				},
			}
			chat, err := bot.GetChat(cic)

			if err != nil {
				// If Telegram doesn't know this ID exists or the user blocked the bot,
				// it's a "hard fail"—remove the link in our DB.
				if err.Error() == agentHasNotStartedBot {
					log.Infow("agent hasn't started bot, pruning ID", "tgid", tgid)
				} else {
					log.Errorw("telegram API error during cleanup", "tgid", tgid, "error", err)
				}

				if gid, gidErr := tgid.Gid(ctx); gidErr == nil {
					_ = gid.RemoveTelegramID(ctx)
				}
				continue
			}

			// We only care about syncing metadata for private chats
			if chat.Type != "private" {
				continue
			}

			// If the username has changed in Telegram, update our local record
			if chat.UserName != "" {
				_ = tgid.SetName(ctx, chat.UserName)
			} else {
				log.Debugw("verified agent has no telegram username", "tgid", tgid)
			}
		}
	}

	log.Info("Telegram state cleanup complete")
	return nil
}
