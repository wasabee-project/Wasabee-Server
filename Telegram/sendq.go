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

// 15/10 with chanSize 30  yeilds 16 second pauses at high load 
// testing 15/8 with chanSize 15 is slow
// ideal would be 20/x
const sendQMessagesPerMinutes = 20
const sendQBurst = 8

// size of 15 is just a "gut feel" value, need to test to determine optimum
const sendQchanSize = 30

// Reverse the logic, channel puts it in the queue, and the queue runner is time-limited...
// The goal is to not have the callers block, but all the blocking to happen on this goprocess
func sendqueueRunner(ctx context.Context) {
	blocked := false
	holdQ := list.New()
	var unblocker <-chan time.Time
	limiter := rate.NewLimiter(rate.Every(time.Minute/sendQMessagesPerMinutes), sendQBurst)

	for {
		select {
		case <-ctx.Done():
			log.Debugw("shutting down message sender", "holdQ len", holdQ.Len())
			return
		case <-unblocker:
			log.Infow("restarting message sender", "holdQ len", holdQ.Len())
			blocked = false
		case msg := <-sendQueue:
			if blocked {
				holdQ.PushBack(msg)
				continue
			}

			if err := limiter.Wait(ctx); err != nil {
				log.Warn(err)
				continue
			}

			if _, err := bot.Send(msg); err != nil {
				errstr := string(err.Error())
				if errstr == "Bad Request: chat not found" { // user has not started the bot or related condition
					// add user to no-send list
					log.Infow("tgid does not have bot started", "msg", msg)
					continue
				}

				log.Error(err)
				if strings.HasPrefix(errstr, "Too Many Requests: retry after ") {
					sleepfor, err := strconv.Atoi(strings.TrimPrefix(errstr, "Too Many Requests: retry after "))
					if err != nil {
						log.Error(err)
						continue
					}
					log.Infow("pausing message sender", "for", sleepfor)
					blocked = true
					holdQ.PushBack(msg)
					unblocker = time.After(time.Duration(sleepfor) * time.Second)
					continue
				}
			}
		}

		// process the holdQ when unblocked or after every message if needed
		// this can probably be moved back into case <-unblocker
		if holdQ.Len() > 0 {
			log.Debugw("running holdQ", "len", holdQ.Len())
			i := 0

			e := holdQ.Front()
			for e != nil {
				n := e.Next()
				msg := holdQ.Remove(e).(tgbotapi.Chattable)
				e = n

				// do not completely fill the sendQueue channel and deadlock this goprocess
				select {
				case sendQueue <- msg: // if channel not full send it
					i++
					if i >= sendQchanSize {
						log.Debug("hit sendQchanSize, backing off")
						unblocker = time.After(30 * time.Second)
						break
					}
				default: // if the channel is full, just bail, and try again in a minute
					log.Debugw("channel full, pausing holdQ until channel has room", "len", holdQ.Len())
					holdQ.PushBack(msg)
					unblocker = time.After(2 * time.Minute)
					e = nil
					break
				}
			}
			log.Debug("outside holdQ for loop")
		}
	}
}
