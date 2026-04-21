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

// just a "gut feel" value, need to test to determine optimum
const sendQchanSize = sendQBurst * 2

// Reverse the logic, channel puts it in the queue, and the queue runner is time-limited...
// The goal is to not have the callers block, but all the blocking to happen on this goprocess
// if this is really the goal, then stuff them in the holdQ as fast as possible and run that queue at the target rate
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
			if holdQ.Len() > 0 {
				log.Debugw("running holdQ", "len", holdQ.Len())
				i := 0

			HQ:
				for e := holdQ.Front(); e != nil; {
					n := e.Next()
					msg := holdQ.Remove(e).(tgbotapi.Chattable)
					e = n

					// do not completely fill the sendQueue channel and deadlock this goprocess
					select {
					case sendQueue <- msg: // if channel not full,  send a few, trigger again to send some more
						i++
						if i >= sendQBurst/2 { // send half-a-burst
							log.Debug("pausing after sending half-a-burst")
							unblocker = time.After((time.Minute / sendQMessagesPerMinutes) * (sendQBurst / 2)) // time it would take to process this block if no congestion
							break HQ
						}
					default: // if the channel is full, just try again later
						log.Debugw("sendQueue full while running holdQ, draining sendQueue into holdQ", "len", holdQ.Len())
						holdQ.PushFront(msg)
						blocked = true                                                      // drain the sendQueue into holdQ
						unblocker = time.After((time.Minute / sendQMessagesPerMinutes) * 2) // restart quickly
						// unblocker = time.After((time.Minute / sendQMessagesPerMinutes) * sendQBurst) // time it would take to process entire sendQueue if no congestion
						break HQ
					}
				}
				log.Debug("outside holdQ for loop")
			}
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
				handleMsgError(msg)
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
	}
}

// move the error handling above into here
// look at way of alerting orig message sender of the fact that the agent does not have the bot started
func handleMsgError(msg tgbotapi.Chattable) {
	switch msg := msg.(type) {
	case tgbotapi.MessageConfig:
		log.Debugw("MessageConfig", "ParseMode", msg.ParseMode, "entities", msg.Entities, "to", msg.ChatID, "ChannelUsername", msg.ChannelUsername)
	default:
		log.Debugw("default msg type", "msg", msg)
	}
}
