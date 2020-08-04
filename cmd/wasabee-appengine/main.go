package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	// "cloud.google.com/go"
	"cloud.google.com/go/storage"

	"github.com/wasabee-project/Wasabee-Server"
	"github.com/wasabee-project/Wasabee-Server/Firebase"
	// "github.com/wasabee-project/Wasabee-Server/RISC"
	"github.com/wasabee-project/Wasabee-Server/PubSub"
	"github.com/wasabee-project/Wasabee-Server/Telegram"
	"github.com/wasabee-project/Wasabee-Server/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func main() {
	// project := os.Getenv("GOOGLE_CLOUD_PROJECT")
	bucket := os.Getenv("BUCKET")
	frontend := os.Getenv("FRONTEND_PATH")
	words := os.Getenv("WORDS_PATH")
	creds := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")

	logconf := wasabee.LogConfiguration{
		Console: true,
	}
	wasabee.SetupLogging(logconf)

	wasabee.Log.Infof("Using Creds: %s", creds)

	ctx := context.Background()

	wasabee.Log.Info("Loading Word List")
	// connect using the App Engine's creds
	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		wasabee.Log.Errorf("Error initializng storage: %s", err)
	}
	b := storageClient.Bucket(bucket)
	w := b.Object(words)
	r, err := w.NewReader(ctx)
	if err != nil {
		wasabee.Log.Errorf("Error loading word list from cloud storage: %s/%s: %s", bucket, words, err)
		panic(err)
	}
	err = wasabee.LoadWordsStream(r)
	if err != nil {
		wasabee.Log.Errorf("Error parsing list: %s", err)
		panic(err)
	}

	wasabee.Log.Info("Loading UI Templates")
	// load the UI templates
	ts, err := wasabee.TemplateConfigAppengine(b, frontend)
	if err != nil {
		wasabee.Log.Errorf("unable to load frontend templates from %s/%s: %s ", bucket, frontend, err)
		panic(err)
	}

	wasabee.Log.Info("Starting Firebase")
	go wasabeefirebase.ServeFirebase(creds)
	defer wasabee.FirebaseClose()
	wasabee.Log.Info("Starting Pub/Sub")
	go wasabeepubsub.StartPubSub(wasabeepubsub.Configuration{
		Cert:    creds,
		Project: "PhDevBin",
	})
	defer wasabee.PubSubClose()

	// Connect to database
	dbstring := os.Getenv("DATABASE")
	wasabee.Log.Infof("Connecting to : %s", dbstring)
	if dbstring == "" {
		dbstring = "user:pass@tcp(localhost)/wasabee"
	}
	err = wasabee.Connect(dbstring)
	if err != nil {
		wasabee.Log.Errorf("Error connecting to database: %s", err)
		panic(err)
	}

	// setup V
	vkey := os.Getenv("VENLONE_API_KEY")
	if vkey != "" {
		wasabee.SetVEnlOne(wasabee.Vconfig{
			APIKey: vkey,
		})
	}

	// setup Rocks
	rockskey := os.Getenv("ENLROCKS_API_KEY")
	if rockskey != "" {
		wasabee.SetEnlRocks(wasabee.Rocksconfig{
			APIKey: rockskey,
		})
	}

	// setup enl.io
	enliokey := os.Getenv("ENLIO_API_KEY")
	if enliokey != "" {
		wasabee.SetENLIO(enliokey)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = ":8080"
	}

	sessionkey := os.Getenv("SESSION_KEY")
	clientid := os.Getenv("OAUTH_CLIENT_ID")
	clientsecret := os.Getenv("OAUTH_CLIENT_SECRET")
	root := os.Getenv("ROOT_URL")
	if root == "" {
		root = "https://cdn.wasabee.rocks"
	}

	// Serve HTTP
	go wasabeehttps.StartAppEngine(wasabeehttps.Configuration{
		ListenHTTPS:  ":" + port,
		FrontendPath: frontend,
		Root:         root,
		OauthConfig: &oauth2.Config{
			ClientID:     clientid,
			ClientSecret: clientsecret,
			Scopes:       []string{"profile email"},
			Endpoint: oauth2.Endpoint{
				AuthURL:   google.Endpoint.AuthURL,
				TokenURL:  google.Endpoint.TokenURL,
				AuthStyle: oauth2.AuthStyleInParams,
			},
		},
		OauthUserInfoURL: "https://www.googleapis.com/oauth2/v2/userinfo",
		CookieSessionKey: sessionkey,
		TemplateSet:      ts,
	})

	// Serve Telegram
	tgkey := os.Getenv("TELEGRAM_API_KEY")
	if tgkey != "" {
		go wasabeetelegram.WasabeeBot(wasabeetelegram.TGConfiguration{
			APIKey:      tgkey,
			HookPath:    "/tg",
			TemplateSet: ts,
		})
	}

	// wait for signal to shut down
	sigch := make(chan os.Signal, 3)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGHUP, os.Interrupt)

	// loop until signal sent
	sig := <-sigch

	wasabee.Log.Info("Shutdown Requested: ", sig)
	if r, _ := wasabee.TGRunning(); r {
		wasabeetelegram.Shutdown()
	}
	_ = wasabeehttps.Shutdown()

	// close database connection
	wasabee.Disconnect()
}
