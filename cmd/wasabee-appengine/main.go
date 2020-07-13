package main

import (
	"os"
	"os/signal"
	"path"
	// "strings"
	"syscall"

	"github.com/wasabee-project/Wasabee-Server"
	"github.com/wasabee-project/Wasabee-Server/Firebase"
	"github.com/wasabee-project/Wasabee-Server/RISC"
	"github.com/wasabee-project/Wasabee-Server/Telegram"
	"github.com/wasabee-project/Wasabee-Server/http"
	"github.com/wasabee-project/Wasabee-Server/PubSub"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func main() {
	certs := "./certs/"
	frontend := os.Getenv("FRONTEND_PATH")

	// Load words
	err := wasabee.LoadWordsFile("word.txt")
	if err != nil {
		wasabee.Log.Errorf("Error loading word list from '%s': %s", "word.txt", err)
	}

	// load the UI templates
	ts, err := wasabee.TemplateConfig(frontend)
	if err != nil {
		wasabee.Log.Errorf("unable to load frontend templates from %s; shutting down", frontend)
		panic(err)
	}

	// Connect to database
	dbstring := os.Getenv("DATABASE")
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
		port = ":8443"
	}

	sessionkey := os.Getenv("SESSION_KEY")
	clientid := os.Getenv("OAUTH_CLIENT_ID")
	clientsecret := os.Getenv("OAUTH_CLIENT_SECRET")
	httpslog := "/dev/null"
	root := os.Getenv("ROOT_URL")
	if root == "" {
		root = "https://zone.wasabee.rocks"
	}

	// Serve HTTPS
		go wasabeehttps.StartHTTP(wasabeehttps.Configuration{
			ListenHTTPS:      port,
			FrontendPath:     frontend,
			Root:             root,
			CertDir:          certs,
			OauthConfig:  &oauth2.Config {
				ClientID: clientid,
				ClientSecret: clientsecret,
				Scopes: []string{"profile email"},
				Endpoint: oauth2.Endpoint {
					AuthURL: google.Endpoint.AuthURL,
					TokenURL: google.Endpoint.TokenURL,
					AuthStyle: oauth2.AuthStyleInParams,
				},
			},
			OauthUserInfoURL: "https://www.googleapis.com/oauth2/v2/userinfo",
			CookieSessionKey: sessionkey,
			Logfile:          httpslog,
			TemplateSet:      ts,
		})

	riscPath := path.Join(certs, "risc.json")
	if _, err := os.Stat(riscPath); err != nil {
		wasabee.Log.Noticef("%s does not exist, not enabling RISC", riscPath)
	} else {
		go risc.RISC(riscPath)
	}

	firebasePath := path.Join(certs, "firebase.json")
	if _, err := os.Stat(firebasePath); err != nil {
		wasabee.Log.Noticef("%s does not exist, not enabling Firebase", firebasePath)
	} else {
		go wasabeefirebase.ServeFirebase(firebasePath)
	}

	pubsubPath := path.Join(certs, "pubsub.json")
	if _, err := os.Stat(pubsubPath); err != nil {
		wasabee.Log.Noticef("%s does not exist, not enabling PubSub", pubsubPath)
	} else {
		go wasabeepubsub.StartPubSub(wasabeepubsub.Configuration{
			Cert: pubsubPath,
			Project: "PhDevBin",
			Topic: "wasabee-main",
		})
	}

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
	if _, err := os.Stat(firebasePath); err == nil {
		wasabee.FirebaseClose()
	}
	if _, err := os.Stat(pubsubPath); err == nil {
		wasabee.PubSubClose()
	}
	if _, err := os.Stat(riscPath); err == nil {
		risc.DisableWebhook()
	}
	if r, _ := wasabee.TGRunning(); r {
		wasabeetelegram.Shutdown()
	}
	_ = wasabeehttps.Shutdown()

	// close database connection
	wasabee.Disconnect()
}
