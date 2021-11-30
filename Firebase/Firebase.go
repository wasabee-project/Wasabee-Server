package wfb

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/auth"
	"firebase.google.com/go/messaging"
	"google.golang.org/api/option"

	"github.com/wasabee-project/Wasabee-Server/log"
)

var config struct {
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

	config.ctx = context.Background()
	opt := option.WithCredentialsFile(keypath)
	app, err := firebase.NewApp(config.ctx, nil, opt)
	if err != nil {
		err := fmt.Errorf("error initializing firebase messaging: %v", err)
		log.Error(err)
		return err
	}

	// make sure we can send messages
	msg, err := app.Messaging(config.ctx)
	if err != nil {
		log.Error(err)
		return err
	}

	client, err := app.Auth(config.ctx)
	if err != nil {
		err := fmt.Errorf("error initializing firebase auth: %v", err)
		log.Error(err)
	}

	config.c = make(chan bool, 1)
	config.running = true
	config.app = app
	config.auth = client
	config.msg = msg

	for b := range config.c {
		log.Debugw("Command on Firebase control channel", "value", b)
	}
	return nil
}

// Close shuts down the channel when done
func Close() {
	if config.running {
		log.Infow("shutdown", "message", "shutting down firebase")
		config.running = false
		close(config.c)
	}
}
