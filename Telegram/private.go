package wtg

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/rocks"
	"github.com/wasabee-project/Wasabee-Server/templates"
)

// processDirectMessage handles messages sent 1-on-1 to the bot
func processDirectMessage(ctx context.Context, inMsg *tgbotapi.Update) error {
	tgid := model.TelegramID(inMsg.Message.From.ID)
	// Use our new context-aware GidV to check verification status
	gid, verified, err := tgid.GidV(ctx)
	if err != nil {
		log.Error(err)
		return err
	}

	// 1. Unknown User: Try auto-login via Rocks, otherwise start OTT init
	if gid == "" {
		log.Infow("unknown user; initializing", "subsystem", "Telegram", "tgusername", inMsg.Message.From.UserName, "tgid", tgid)

		// Check if they exist on Enl.Rocks
		fgid, err := firstlogin(ctx, tgid, inMsg.Message.From.UserName)
		if fgid != "" && err == nil {
			text, _ := templates.ExecuteLang("InitTwoSuccess", inMsg.Message.From.LanguageCode, nil)
			msg := tgbotapi.NewMessage(inMsg.Message.Chat.ID, text)
			msg.ParseMode = "HTML"
			msg.ReplyMarkup = baseKbd
			sendQueue <- msg
			return nil
		}

		// Fallback to manual One-Time-Token initialization
		return newUserInit(ctx, inMsg)
	}

	// 2. Unverified User: They've started the process, need the verification code
	if !verified {
		log.Infow("verifying Telegram user", "subsystem", "Telegram", "tgusername", inMsg.Message.From.UserName, "tgid", tgid)
		return newUserVerify(ctx, inMsg)
	}

	// 3. Verified User: Standard command processing
	return processVerifiedDM(ctx, inMsg, gid)
}

func processVerifiedDM(ctx context.Context, inMsg *tgbotapi.Update, gid model.GoogleID) error {
	msg := tgbotapi.NewMessage(inMsg.Message.Chat.ID, "")
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = baseKbd

	// Update cached Telegram name if it changed
	if inMsg.Message.From.UserName != "" {
		tgid := model.TelegramID(inMsg.Message.From.ID)
		_ = tgid.SetName(ctx, inMsg.Message.From.UserName)
	}

	if inMsg.Message.IsCommand() {
		switch inMsg.Message.Command() {
		case "start":
			msg.Text, _ = templates.ExecuteLang("start", inMsg.Message.From.LanguageCode, nil)
		case "help":
			msg.Text, _ = templates.ExecuteLang("default", inMsg.Message.From.LanguageCode, botCommands)
		default:
			msg.Text, _ = templates.ExecuteLang("default", inMsg.Message.From.LanguageCode, botCommands)
		}
	} else if inMsg.Message.Location != nil {
		lat := strconv.FormatFloat(inMsg.Message.Location.Latitude, 'f', -1, 64)
		lon := strconv.FormatFloat(inMsg.Message.Location.Longitude, 'f', -1, 64)
		if err := gid.SetLocation(ctx, lat, lon); err != nil {
			log.Error(err)
		}
		msg.Text = "Location updated."
	} else {
		// Default response for non-command text
		msg.Text, _ = templates.ExecuteLang("default", inMsg.Message.From.LanguageCode, botCommands)
	}

	if msg.Text != "" {
		sendQueue <- msg
	}
	return nil
}

func firstlogin(ctx context.Context, tgid model.TelegramID, name string) (model.GoogleID, error) {
	// rocks.Search now context-aware
	agent, err := rocks.Search(ctx, fmt.Sprint(tgid))
	if err != nil || agent.Gid == "" {
		return "", err
	}

	gid := model.GoogleID(agent.Gid)
	// If the agent exists in Rocks but is new to our DB
	if !gid.Valid(ctx) {
		if err := gid.FirstLogin(ctx); err != nil {
			return "", err
		}
	}

	if err := gid.SetTelegramID(ctx, tgid, name); err != nil {
		return gid, err
	}

	return gid, nil
}

func newUserInit(ctx context.Context, inMsg *tgbotapi.Update) error {
	ott := model.OneTimeToken(parseToken(inMsg))
	tid := model.TelegramID(inMsg.Message.From.ID)

	var text string
	if err := tid.InitAgent(ctx, inMsg.Message.From.UserName, ott); err != nil {
		text, _ = templates.ExecuteLang("InitOneFail", inMsg.Message.From.LanguageCode, nil)
	} else {
		text, _ = templates.ExecuteLang("InitOneSuccess", inMsg.Message.From.LanguageCode, nil)
	}

	msg := tgbotapi.NewMessage(inMsg.Message.Chat.ID, text)
	msg.ParseMode = "HTML"
	sendQueue <- msg
	return nil
}

func newUserVerify(ctx context.Context, inMsg *tgbotapi.Update) error {
	token := parseToken(inMsg)
	tid := model.TelegramID(inMsg.Message.From.ID)

	var text string
	if err := tid.VerifyAgent(ctx, token); err != nil {
		text, _ = templates.ExecuteLang("InitTwoFail", inMsg.Message.From.LanguageCode, nil)
	} else {
		text, _ = templates.ExecuteLang("InitTwoSuccess", inMsg.Message.From.LanguageCode, nil)
	}

	msg := tgbotapi.NewMessage(inMsg.Message.Chat.ID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = baseKbd
	sendQueue <- msg
	return nil
}

// helper to grab token from "/command token" or just "token"
func parseToken(inMsg *tgbotapi.Update) string {
	input := inMsg.Message.Text
	if inMsg.Message.IsCommand() {
		tokens := strings.Split(input, " ")
		if len(tokens) >= 2 {
			return strings.TrimSpace(tokens[1])
		}
	}
	return strings.TrimSpace(input)
}
