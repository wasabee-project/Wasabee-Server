package wtg

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gorilla/mux"

	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/templates"
	"github.com/wasabee-project/Wasabee-Server/util"
)

var (
	baseKbd   tgbotapi.ReplyKeyboardMarkup
	upChan    chan tgbotapi.Update
	sendQueue chan tgbotapi.Chattable
	hook      string
	bot       *tgbotapi.BotAPI
)

// Start is called from main() to start the bot.
func Start(ctx context.Context) {
	c := config.Get().Telegram

	if c.APIKey == "" {
		log.Infow("startup", "subsystem", "Telegram", "message", "Telegram API key not set; not starting")
		return
	}

	baseKbd = keyboards()
	upChan = make(chan tgbotapi.Update, 100)
	sendQueue = make(chan tgbotapi.Chattable, sendQchanSize)

	// In v5, SetLogger takes a standard logger or nil
	// _ = tgbotapi.SetLogger(log.NewNoopLogger())

	var err error
	bot, err = tgbotapi.NewBotAPI(c.APIKey)
	if err != nil {
		log.Error(err)
		return
	}

	log.Infow("startup", "subsystem", "Telegram", "message", "authorized to Telegram as "+bot.Self.UserName)
	config.TGSetBot(bot.Self.UserName, int(bot.Self.ID))

	// Set initial webhook
	hook = util.GenerateName()
	if err := setWebhook(hook); err != nil {
		log.Error(err)
		return
	}

	// Subrouter for webhooks
	subrouter := config.Subrouter(c.HookPath)
	subrouter.HandleFunc("/{hook}", webhookHandler).Methods("POST")

	// Register with messaging subsystem - Note: These need to be wrapped to handle ctx
	messaging.RegisterMessageBus("Telegram", messaging.Bus{
		SendMessage: func(ctx context.Context, g messaging.GoogleID, m string) (bool, error) {
			return sendMessage(ctx, g, m)
		},
		SendTarget: func(ctx context.Context, g messaging.GoogleID, t messaging.Target) error {
			return sendTarget(ctx, g, t)
		},
		// ... other bus methods
	})

	// Run workers
	go sendqueueRunner(ctx)
	if c.CleanOnStartup {
		go cleanup(ctx)
	}

	setupCommands(ctx)

	// Main update loop
	i := 1
	for {
		select {
		case <-ctx.Done():
			shutdown()
			return
		case update := <-upChan:
			if err := runUpdate(ctx, update); err != nil {
				log.Error(err)
			}

			// Rotate webhook every 100 updates to keep the endpoint "moving"
			if i%100 == 0 {
				i = 1
				hook = util.GenerateName()
				if err := setWebhook(hook); err != nil {
					log.Error(err)
				}
			}
			i++
		}
	}
}

func setWebhook(path string) error {
	c := config.Get().Telegram
	whurl := fmt.Sprintf("%s%s/%s", config.GetWebroot(), c.HookPath, path)
	wh, err := tgbotapi.NewWebhook(whurl)
	if err != nil {
		return err
	}
	_, err = bot.Request(wh)
	return err
}

func shutdown() {
	log.Infow("shutdown", "subsystem", "Telegram", "message", "cleaning up telegram")
	// Remove webhook on clean shutdown
	wh, _ := tgbotapi.NewWebhook("")
	_, _ = bot.Request(wh)
}

func runUpdate(ctx context.Context, update tgbotapi.Update) error {
	if update.CallbackQuery != nil {
		msg, err := callback(ctx, &update)
		if err != nil {
			return err
		}
		if msg != nil {
			sendQueue <- msg
		}
		return nil
	}

	if update.Message != nil {
		if update.Message.Chat.IsPrivate() {
			return processDirectMessage(ctx, &update)
		}
		return processChatMessage(ctx, &update)
	}

	if update.EditedMessage != nil && update.EditedMessage.Location != nil {
		return liveLocationUpdate(ctx, &update)
	}

	return nil
}

// keyboards provides the primary UI
func keyboards() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButtonLocation("Send Location"),
			tgbotapi.NewKeyboardButton("Tasks"),
		),
	)
}

func sendMessage(ctx context.Context, g messaging.GoogleID, message string) (bool, error) {
	gid := model.GoogleID(g)
	tgid, err := gid.TelegramID(ctx)
	if err != nil || tgid == 0 {
		return false, err
	}

	msg := tgbotapi.NewMessage(int64(tgid), message)
	msg.ParseMode = "HTML"
	msg.DisableWebPagePreview = true

	sendQueue <- msg
	return true, nil
}

func sendTarget(ctx context.Context, g messaging.GoogleID, target messaging.Target) error {
	gid := model.GoogleID(g)
	tgid, err := gid.TelegramID(ctx)
	if err != nil || tgid == 0 {
		return err
	}

	templateData := struct {
		messaging.Target
		Lon string // template expects .Lon
	}{
		Target: target,
		Lon:    target.Lng,
	}

	text, err := templates.Execute("target", templateData)
	if err != nil {
		log.Error(err)
		text = fmt.Sprintf("target: %s @ %s %s", target.Name, target.Lat, target.Lng)
	}

	msg := tgbotapi.NewMessage(int64(tgid), text)
	msg.ParseMode = "HTML"
	sendQueue <- msg
	return nil
}

// webhookHandler bridges the HTTP POST to the upChan update loop
func webhookHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	if vars["hook"] != hook {
		log.Warnw("unauthorized webhook access", "received", vars["hook"])
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	update, err := bot.HandleUpdate(r)
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	upChan <- *update
	w.WriteHeader(http.StatusOK)
}
