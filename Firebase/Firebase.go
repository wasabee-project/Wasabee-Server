package WasabeeFirebase

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go"
	// "firebase.google.com/go/auth"
	"firebase.google.com/go/messaging"

	"google.golang.org/api/option"

	"github.com/wasabee-project/Wasabee-Server"
)

var config struct {
	app	*firebase.App
	msg	*messaging.Client
}

// ServeFirebase is the main startup function for the Firebase integration
func ServeFirebase(keypath string) error {
	wasabee.Log.Debug("starting Firebase")

	ctx := context.Background()
	opt := option.WithCredentialsFile(keypath)
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		err := fmt.Errorf("error initializing firebase: %v", err)
		return err
	}
	config.app = app

	// make sure we can authenticate
	if auth, err := app.Auth(ctx); auth == nil || err != nil {
		err = fmt.Errorf("Auth() = (%v, %v); want (auth, nil)", auth, err)
		return err
	}

	// make sure we can send messages
	msg, err := app.Messaging(ctx)
	if msg == nil || err != nil {
		err = fmt.Errorf("Messaging() = (%v, %v); want (iid, nil)", msg, err)
		return err
	}
	config.msg = msg

	return nil
}
