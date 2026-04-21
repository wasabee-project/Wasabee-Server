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

// newResponse initializes a standard message with HTML and the default keyboard
func newResponse(chatID int64) tgbotapi.MessageConfig {
	msg := tgbotapi.NewMessage(chatID, "")
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = baseKbd
	return msg
}

func processDirectMessage(ctx context.Context, inMsg *tgbotapi.Update) error {
	if inMsg.Message == nil {
		return nil
	}

	from := inMsg.Message.From
	tgid := model.TelegramID(from.ID)
	gid, verified, err := tgid.GidV(ctx)
	if err != nil {
		log.Error(err)
		return err
	}

	// 1. User is unknown to the system
	if gid == "" {
		log.Infow("unknown user; initializing", "subsystem", "Telegram", "tgusername", from.UserName, "tgid", tgid)

		fgid, err := firstlogin(ctx, tgid, from.UserName)
		if err == nil && fgid != "" {
			msg := newResponse(inMsg.Message.Chat.ID)
			msg.Text, _ = templates.ExecuteLang("InitTwoSuccess", from.LanguageCode, nil)
			sendQueue <- msg
			return nil
		}

		// Not found in Rocks/V, start manual OTT process
		msg, err := newUserInit(ctx, inMsg)
		if err != nil {
			log.Error(err)
		}
		sendQueue <- msg
		return nil
	}

	// 2. User is known but not verified
	if !verified {
		log.Infow("verifying Telegram user", "subsystem", "Telegram", "tgusername", from.UserName, "tgid", tgid)
		msg, err := newUserVerify(ctx, inMsg)
		if err != nil {
			log.Error(err)
		}
		sendQueue <- msg
		return nil
	}

	// 3. User is verified, process regular commands
	return processMessage(ctx, inMsg, gid)
}

func processMessage(ctx context.Context, inMsg *tgbotapi.Update, gid model.GoogleID) error {
	msg := newResponse(inMsg.Message.Chat.ID)
	from := inMsg.Message.From

	// Update telegram name in DB if it changed
	if from.UserName != "" {
		tgid := model.TelegramID(from.ID)
		if err := tgid.SetName(ctx, from.UserName); err != nil {
			log.Error(err)
		}
	}

	// Logic Switch: Priority 1: Commands, Priority 2: Location, Priority 3: Text
	switch {
	case inMsg.Message.IsCommand():
		switch inMsg.Message.Command() {
		case "start":
			msg.Text, _ = templates.ExecuteLang("start", from.LanguageCode, nil)
		case "help":
			msg.Text, _ = templates.ExecuteLang("default", from.LanguageCode, commands)
		default:
			msg.Text, _ = templates.ExecuteLang("default", from.LanguageCode, commands)
		}

	case inMsg.Message.Location != nil:
		log.Debugw("processing location", "subsystem", "Telegram", "GID", gid)
		lat := strconv.FormatFloat(inMsg.Message.Location.Latitude, 'f', -1, 64)
		lon := strconv.FormatFloat(inMsg.Message.Location.Longitude, 'f', -1, 64)
		if err := gid.SetLocation(ctx, lat, lon); err != nil {
			log.Error(err)
		}
		// Optional: could send a confirmation or just let the keyboard stay
		return nil

	case inMsg.Message.Text != "":
		if strings.EqualFold(inMsg.Message.Text, "wasabee") {
			msg.Text = "wasabee rocks"
		} else {
			// No recognized text, show help
			msg.Text, _ = templates.ExecuteLang("default", from.LanguageCode, commands)
		}
	}

	if msg.Text != "" {
		sendQueue <- msg
	}
	return nil
}

func firstlogin(ctx context.Context, tgid model.TelegramID, name string) (model.GoogleID, error) {
	agent, err := rocks.Search(ctx, fmt.Sprint(tgid))
	if err != nil {
		return "", err
	}

	if agent.Gid != "" {
		gid := model.GoogleID(agent.Gid)
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
	return "", nil
}

func newUserInit(ctx context.Context, inMsg *tgbotapi.Update) (*tgbotapi.MessageConfig, error) {
	msg := newResponse(inMsg.Message.Chat.ID)
	from := inMsg.Message.From

	var ott string
	if inMsg.Message.IsCommand() {
		val := inMsg.Message.CommandArguments()
		ott = strings.TrimSpace(val)
	} else {
		ott = strings.TrimSpace(inMsg.Message.Text)
	}

	tid := model.TelegramID(from.ID)
	err := tid.InitAgent(ctx, from.UserName, model.OneTimeToken(ott))

	templateName := "InitOneSuccess"
	if err != nil {
		log.Error(err)
		templateName = "InitOneFail"
	}

	msg.Text, _ = templates.ExecuteLang(templateName, from.LanguageCode, nil)
	return &msg, err
}

func newUserVerify(ctx context.Context, inMsg *tgbotapi.Update) (*tgbotapi.MessageConfig, error) {
	msg := newResponse(inMsg.Message.Chat.ID)
	from := inMsg.Message.From

	var token string
	if inMsg.Message.IsCommand() {
		val := inMsg.Message.CommandArguments()
		token = strings.TrimSpace(val)
	} else {
		token = strings.TrimSpace(inMsg.Message.Text)
	}

	tid := model.TelegramID(from.ID)
	err := tid.VerifyAgent(ctx, token)

	templateName := "InitTwoSuccess"
	if err != nil {
		log.Error(err)
		templateName = "InitTwoFail"
	}

	msg.Text, _ = templates.ExecuteLang(templateName, from.LanguageCode, nil)
	return &msg, err
}
