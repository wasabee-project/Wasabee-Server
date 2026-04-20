package wtg

import (
	"container/list"
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golang.org/x/time/rate"

	"github.com/wasabee-project/Wasabee-Server/log"
)

const (
	sendQMessagesPerMinute = 20
	sendQBurst             = 8
	sendQchanSize          = 30
)

// sendqueueRunner processes the outgoing message queue with rate limiting
func sendqueueRunner(ctx context.Context) {
	holdQ := list.New()
	limiter := rate.NewLimiter(rate.Every(time.Minute/sendQMessagesPerMinute), sendQBurst)

	// A timer to wake up and check the holdQ when we aren't being pushed a new message
	processTimer := time.NewTicker(100 * time.Millisecond)
	defer processTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Infow("shutting down message sender", "subsystem", "Telegram", "remaining", holdQ.Len())
			return

		case msg, ok := <-sendQueue:
			if !ok {
				return
			}
			holdQ.PushBack(msg)

		case <-processTimer.C:
			// If we have nothing to send, just wait
			if holdQ.Len() == 0 {
				continue
			}

			// Peek at the first item
			element := holdQ.Front()
			msg := element.Value.(tgbotapi.Chattable)

			// Check rate limit (non-blocking check if we can proceed)
			if !limiter.Allow() {
				continue
			}

			// Try to send
			if _, err := bot.Send(msg); err != nil {
				errStr := err.Error()

				// Handle Telegram Rate Limiting (429)
				if strings.Contains(errStr, "Too Many Requests: retry after ") {
					secondsStr := strings.TrimPrefix(errStr, "Too Many Requests: retry after ")
					if seconds, err := strconv.Atoi(secondsStr); err == nil {
						log.Warnw("telegram rate limit hit", "wait_seconds", seconds)
						// Back off: wait the requested time plus a small buffer
						time.Sleep(time.Duration(seconds)*time.Second + (500 * time.Millisecond))
					}
					continue // Leave it in the queue and try again
				}

				// Handle "User Blocked Bot" or "Chat Not Found"
				if strings.Contains(errStr, "chat not found") || strings.Contains(errStr, "bot was blocked") {
					log.Infow("cannot send to user/chat; removing from queue", "error", errStr, "msg", msg)
					holdQ.Remove(element)
					continue
				}

				log.Errorw("telegram send error", "error", err, "msg", msg)
				// For other errors, remove to prevent infinite retry loops
				holdQ.Remove(element)
			} else {
				// Successfully sent
				holdQ.Remove(element)
			}
		}
	}
}
