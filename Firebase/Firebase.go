package wfb

import (
	"context"
	"fmt"
	"os"
	"path"

	firebase "firebase.google.com/go"
	// "firebase.google.com/go/auth"
	"firebase.google.com/go/messaging"
	"google.golang.org/api/option"

	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	wm "github.com/wasabee-project/Wasabee-Server/messaging"
)

var msg *messaging.Client
var fbctx context.Context

// app     *firebase.App
// auth    *auth.Client

// Start is the main startup function for the Firebase subsystem
func Start(ctx context.Context) error {
	c := config.Get()
	keypath := path.Join(c.Certs, c.FirebaseKey)

	if _, err := os.Stat(keypath); err != nil {
		log.Debugw("firebase key does not exist, not starting", "key", keypath)
		return nil
	}

	log.Infow("startup", "subsystem", "Firebase", "version", firebase.Version, "message", "Firebase starting", "key", keypath)

	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsFile(keypath))
	if err != nil {
		err := fmt.Errorf("error initializing firebase messaging: %v", err)
		log.Error(err)
		return err
	}

	msg, err = app.Messaging(ctx)
	if err != nil {
		log.Error(err)
		return err
	}

	// not currently used
	/* auth, err := app.Auth(ctx)
	if err != nil {
		err := fmt.Errorf("error initializing firebase auth: %v", err)
		log.Error(err)
	} */

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

	fbctx = ctx
	config.SetFirebaseRunning(true)

	// there is no reason to stay running now -- this costs nothing
	select {
	case <-ctx.Done():
		break
	}

	log.Infow("Shutdown", "message", "Firebase Shutting down")
	config.SetFirebaseRunning(false)
	return nil
}
