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

	wm.RegisterMessageBus("firebase", wm.Bus{
		SendMessage:      sendMessage,
		SendTarget:       sendTarget,
		SendAnnounce:     sendAnnounce,
		AddToRemote:      addToRemote,
		RemoveFromRemote: removeFromRemote,
		// SendAssignment: sendAssignment,
		AgentDeleteOperation: agentDeleteOperation,
		DeleteOperation:      deleteOperation,
	})

	fbctx = ctx
	config.SetFirebaseRunning(true)

	// there is no reason to stay running now -- this costs nothing
	<-ctx.Done()

	log.Infow("Shutdown", "message", "Firebase Shutting down")
	config.SetFirebaseRunning(false)
	return nil
}
