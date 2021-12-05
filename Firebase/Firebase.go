package wfb

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/auth"
	"firebase.google.com/go/messaging"
	"google.golang.org/api/option"

	"github.com/wasabee-project/Wasabee-Server/log"
	wm "github.com/wasabee-project/Wasabee-Server/messaging"
)

var c struct {
	running bool
	c       chan bool
	app     *firebase.App
	msg     *messaging.Client
	auth    *auth.Client
	ctx     context.Context
}

// Serve is the main startup function for the Firebase subsystem
func Serve(keypath string) error {
	log.Infow("startup", "subsystem", "Firebase", "version", firebase.Version, "message", "Firebase starting")

	c.ctx = context.Background()
	opt := option.WithCredentialsFile(keypath)
	app, err := firebase.NewApp(c.ctx, nil, opt)
	if err != nil {
		err := fmt.Errorf("error initializing firebase messaging: %v", err)
		log.Error(err)
		return err
	}

	// make sure we can send messages
	msg, err := app.Messaging(c.ctx)
	if err != nil {
		log.Error(err)
		return err
	}

	client, err := app.Auth(c.ctx)
	if err != nil {
		err := fmt.Errorf("error initializing firebase auth: %v", err)
		log.Error(err)
	}

	c.c = make(chan bool, 1)
	c.running = true
	c.app = app
	c.auth = client
	c.msg = msg

	wm.RegisterMessageBus("firebase", wm.Bus{
		SendMessage:      SendMessage,
		SendTarget:       SendTarget,
		SendAnnounce:     SendAnnounce,
		AddToRemote:      AddToRemote,
		RemoveFromRemote: RemoveFromRemote,
		// SendAssignment: SendAssignment,
		AgentDeleteOperation: AgentDeleteOperation,
		DeleteOperation:      DeleteOperation,
	})

	for b := range c.c {
		log.Debugw("Command on Firebase control channel", "value", b)
	}
	return nil
}

// Close shuts down the channel when done
func Close() {
	if c.running {
		log.Infow("shutdown", "message", "shutting down firebase")
		c.running = false
		close(c.c)
	}
}
