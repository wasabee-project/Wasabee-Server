package wtg

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	// "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golang.org/x/time/rate"

	"github.com/wasabee-project/Wasabee-Server/log"
)

// Telegram limits: 30 msgs/sec global, 20 msgs/min for broadcasts
// We use 20/min to stay very safe and avoid the "Too Many Requests" churn.
const (
	sendQMessagesPerMinute = 20
	sendQBurst             = 8
	sendQchanSize          = 15
)

func sendqueueRunner(ctx context.Context) {
	// Rate limiter: 1 token every 3 seconds (20 per minute), with a burst capacity of 8
	limiter := rate.NewLimiter(rate.Every(time.Minute/sendQMessagesPerMinute), sendQBurst)

	for {
		select {
		case <-ctx.Done():
			log.Debug("shutting down telegram sendqueueRunner")
			return

		case msg := <-sendQueue:
			// 1. Wait for local rate limit
			if err := limiter.Wait(ctx); err != nil {
				return
			}

		RETRY:
			// 2. Attempt the send
			if _, err := bot.Send(msg); err != nil {
				errStr := err.Error()

				// 3. Handle 429 Too Many Requests (Manual Backoff)
				if strings.HasPrefix(errStr, "Too Many Requests: retry after ") {
					secondsStr := strings.TrimPrefix(errStr, "Too Many Requests: retry after ")
					if seconds, err := strconv.Atoi(secondsStr); err == nil {
						log.Infow("telegram forced backoff", "seconds", seconds)

						select {
						case <-ctx.Done():
							return
						case <-time.After(time.Duration(seconds) * time.Second):
							goto RETRY // Re-attempt the exact same message
						}
					}
				}

				// 4. Handle "Dead" Chats (User blocked bot, etc)
				if errStr == "Bad Request: chat not found" || strings.Contains(strings.ToLower(errStr), "forbidden") {
					log.Debugw("dropping message: chat invalid/blocked", "error", errStr, "msg", msg)
					continue
				}

				// 5. Generic Error Logging
				log.Errorw("telegram send error", "error", err, "msgType", fmt.Sprintf("%T", msg))
			}
		}
	}
}
