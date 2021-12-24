package main

import (
	"context"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"

	"cloud.google.com/go/profiler"
	"google.golang.org/api/option"
	// "github.com/google/pprof"

	"github.com/urfave/cli"

	"github.com/wasabee-project/Wasabee-Server/Firebase"
	// "github.com/wasabee-project/Wasabee-Server/PubSub"
	"github.com/wasabee-project/Wasabee-Server/RISC"
	"github.com/wasabee-project/Wasabee-Server/Telegram"
	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/generatename"
	"github.com/wasabee-project/Wasabee-Server/http"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/rocks"
	"github.com/wasabee-project/Wasabee-Server/templates"
	"github.com/wasabee-project/Wasabee-Server/v"

	"golang.org/x/oauth2"

	"go.uber.org/zap"
	"golang.org/x/oauth2/google"
)

const version = "0.99.1"

var flags = []cli.Flag{
	cli.StringFlag{
		Name: "database, d", EnvVar: "DATABASE", Value: "wasabee:GoodPassword@tcp(localhost)/wasabee",
		Usage: "MySQL/MariaDB connection string. It is recommended to pass this parameter as an environment variable"},
	cli.StringFlag{
		Name: "certs", EnvVar: "CERTDIR", Value: "./certs/",
		Usage: "Directory where HTTPS certificates are stored"},
	cli.StringFlag{
		Name: "root, r", EnvVar: "ROOT_URL", Value: "https://wasabee.phtiv.com",
		Usage: "The path under which the application will be reachable from the internet"},
	cli.StringFlag{
		Name: "wordlist", EnvVar: "WORD_LIST", Value: "eff_large_wordlist.txt",
		Usage: "Word list used for random slug generation"},
	cli.StringFlag{
		Name: "log", EnvVar: "LOGFILE", Value: "logs/wasabee.log",
		Usage: "output log file"},
	cli.StringFlag{
		Name: "https", EnvVar: "HTTPS_LISTEN", Value: ":443",
		Usage: "HTTPS listen address"},
	cli.StringFlag{
		Name: "httpslog", EnvVar: "HTTPS_LOGFILE", Value: "logs/wasabee-https.log",
		Usage: "HTTPS log file."},
	cli.StringFlag{
		Name: "frontend-path, p", EnvVar: "FRONTEND_PATH", Value: "./frontend",
		Usage: "Location of the frontend files."},
	cli.StringFlag{
		Name: "oauth-clientid", EnvVar: "OAUTH_CLIENT_ID", Value: "UNSET",
		Usage: "OAuth ClientID. It is recommended to pass this parameter as an environment variable"},
	cli.StringFlag{
		Name: "oauth-secret", EnvVar: "OAUTH_CLIENT_SECRET", Value: "UNSET",
		Usage: "OAuth Client Secret. It is recommended to pass this parameter as an environment variable"},
	cli.StringFlag{
		Name: "oauth-authurl", EnvVar: "OAUTH_AUTH_URL", Value: google.Endpoint.AuthURL,
		Usage: "OAuth Auth URL. Defaults to Google's well-known auth url"},
	cli.StringFlag{
		Name: "oauth-tokenurl", EnvVar: "OAUTH_TOKEN_URL", Value: google.Endpoint.TokenURL,
		Usage: "OAuth Token URL. Defaults to Google's well-known token url"},
	cli.StringFlag{
		Name: "oauth-userinfo", EnvVar: "OAUTH_USERINFO_URL", Value: "https://www.googleapis.com/oauth2/v2/userinfo",
		Usage: "OAuth userinfo URL. Defaults to Google's well-known userinfo url"},
	cli.StringFlag{
		Name: "sessionkey", EnvVar: "SESSION_KEY", Value: "",
		Usage: "Session Key (32 char, random). It is recommended to pass this parameter as an environment variable"},
	cli.StringFlag{
		Name: "tgkey", EnvVar: "TELEGRAM_API_KEY", Value: "",
		Usage: "Telegram API Key. It is recommended to pass this parameter as an environment variable"},
	cli.StringFlag{
		Name: "venlonekey", EnvVar: "VENLONE_API_KEY", Value: "",
		Usage: "V.enl.one API Key. It is recommended to pass this parameter as an environment variable"},
	cli.StringFlag{
		Name: "venloneapiurl", EnvVar: "VENLONE_API_URL", Value: "",
		Usage: "V.enl.one API URL. Defaults to v.enl.one well-known URL"},
	cli.StringFlag{
		Name: "venlonestatusurl", EnvVar: "VENLONE_STATUS_URL", Value: "",
		Usage: "V.enl.one Status URL. Defaults to status.enl.one well-known URL"},
	cli.StringFlag{
		Name: "enlrockskey", EnvVar: "ENLROCKS_API_KEY", Value: "",
		Usage: "enl.rocks API Key. It is recommended to pass this parameter as an environment variable"},
	cli.StringFlag{
		Name: "enlrockscommurl", EnvVar: "ENLROCKS_COMM_URL", Value: "",
		Usage: "enl.rocks Community API URL. Defaults to the enl.rocks well-known URL"},
	cli.StringFlag{
		Name: "enlrocksstatusurl", EnvVar: "ENLROCKS_STATUS_URL", Value: "",
		Usage: "enl.rocks Status API URL. Defaults to the enl.rocks well-known URL"},
	cli.StringFlag{
		Name: "jwkpriv", EnvVar: "JWK_SINGER_PATH", Value: "certs/jwkpriv.json",
		Usage: "file containing the json-encoded JWK used to sign JWT"},
	cli.StringFlag{
		Name: "jwkpub", EnvVar: "JWK_VERIFIER_PATH", Value: "certs/jwkpub.json",
		Usage: "file containing the json-encoded JWK used to verify JWT"},
	cli.StringFlag{
		Name: "webui", EnvVar: "WEBUIURL", Value: "https://wasabee-project.github.io/Wasabee-WebUI/",
		Usage: "URL to the WebUI"},
	cli.BoolFlag{
		Name: "debug", EnvVar: "DEBUG",
		Usage: "Show (a lot) more output"},
	cli.BoolFlag{
		Name: "longtimeouts", EnvVar: "LONG_TIMEOUTS",
		Usage: "Increase timeouts to 1 hour. (should only be used while debugging)"},
	cli.BoolFlag{
		Name:  "help, h",
		Usage: "Shows this help, then exits"},
}

func main() {
	app := cli.NewApp()

	app.Name = "wasabee-server"
	app.Version = version
	app.Usage = "Wasabee Server"
	app.Authors = []cli.Author{
		{
			Name:  "Scot C. Bontrager",
			Email: "scot@wasabee.rocks",
		},
	}
	app.Copyright = "© Scot C. Bontrager"
	app.HelpName = "wasabee"
	app.Flags = flags
	app.HideHelp = true
	cli.AppHelpTemplate = strings.Replace(cli.AppHelpTemplate, "GLOBAL OPTIONS:", "OPTIONS:", 1)

	app.Action = run

	// f, _ := os.Create("logs/profile")
	// pprof.StartCPUProfile(f)
	// defer pprof.StopCPUProfile()

	_ = app.Run(os.Args)
}

func run(c *cli.Context) error {
	project := os.Getenv("GCP_PROJECT")
	creds := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")

	if c.Bool("help") {
		_ = cli.ShowAppHelp(c)
		return nil
	}

	logconf := log.Configuration{
		Console:            true,
		ConsoleLevel:       zap.InfoLevel,
		FilePath:           c.String("log"),
		FileLevel:          zap.InfoLevel,
		GoogleCloudProject: project,
		GoogleCloudCreds:   creds,
	}
	if c.Bool("debug") {
		logconf.ConsoleLevel = zap.DebugLevel
	}
	log.SetupLogging(logconf)

	if creds != "" {
		if _, err := os.Stat(creds); err == nil {
			opts := option.WithCredentialsFile(creds)
			cfg := profiler.Config{
				Service:        "wasabee",
				ServiceVersion: version,
				ProjectID:      "phdevbin",
			}
			if err := profiler.Start(cfg, opts); err != nil {
				log.Errorw("startup", "message", "unable to start profiler", "error", err)
			} else {
				log.Infow("startup", "message", "starting gcloud profiling")
			}
		}
	}

	// Load words
	err := generatename.LoadWordsFile(c.String("wordlist"))
	if err != nil {
		log.Fatalw("startup", "message", "Error loading word list", "wordlist", c.String("wordlist"), "error", err.Error())
	}

	// load the UI templates
	ts, err := templates.Startup(c.String("frontend-path"))
	if err != nil {
		log.Fatalw("startup", "message", "unable to load frontend templates; shutting down", "path", c.String("frontend-path"), "error", err.Error())
	}

	// Connect to database
	err = model.Connect(c.String("database"))
	if err != nil {
		log.Fatalw("startup", "message", "Error connecting to database", "error", err.Error())
	}

	// setup V
	if c.String("venlonekey") != "" {
		v.Startup(c.String("venlonekey"))
	}
	/*
			APIEndpoint:    c.String("venloneapiurl"),
		 	StatusEndpoint: c.String("venlonestatusurl"),
	*/

	// setup Rocks
	if c.String("enlrockskey") != "" {
		// rocks.Config.APIKey = c.String("enlrockskey")
		rocks.Start(c.String("enlrockskey"))
	}
	/* if c.String("enlrockscommurl") != "" {
		rocks.Config.CommunityEndpoint = c.String("enlrockscommurl")
	} */

	config.SetupJWK(c.String("jwkpriv"), c.String("jwkpub"))

	// Serve HTTPS
	if c.String("https") != "none" {
		go wasabeehttps.StartHTTP(wasabeehttps.Configuration{
			ListenHTTPS:  c.String("https"),
			FrontendPath: c.String("frontend-path"),
			Root:         c.String("root"),
			CertDir:      c.String("certs"),
			OauthConfig: &oauth2.Config{
				ClientID:     c.String("oauth-clientid"),
				ClientSecret: c.String("oauth-secret"),
				Scopes:       []string{"profile email"},
				Endpoint: oauth2.Endpoint{
					AuthURL:   c.String("oauth-authurl"),
					TokenURL:  c.String("oauth-tokenurl"),
					AuthStyle: oauth2.AuthStyleInParams,
				},
			},
			OauthUserInfoURL: c.String("oauth-userinfo"),
			CookieSessionKey: c.String("sessionkey"),
			Logfile:          c.String("httpslog"),
			WebUIurl:         c.String("webui"),
		})
	}

	// this one should not use GOOGLE_APPLICATION_CREDENTIALS because it requires odd privs
	riscPath := path.Join(c.String("certs"), "risc.json")
	if _, err := os.Stat(riscPath); err != nil {
		log.Infow("startup", "message", "credentials do not exist, not enabling RISC", "credentials", riscPath)
	} else {
		go risc.RISC(context.Background(), riscPath)
	}

	// requires Firebase SDK and PubSub publisher & subscriber access
	// XXX should have CLI/env args for these
	if creds != "" {
		go wfb.Serve(creds)
		/* go wps.Start(wps.Configuration{
			Cert:    creds,
			Project: project,
		}) */
	}

	// Serve Telegram
	if c.String("tgkey") != "" {
		go wtg.WasabeeBot(&wtg.Config{
			APIKey:      c.String("tgkey"),
			HookPath:    "/tg",
			TemplateSet: ts,
		})
	}

	// wait for signal to shut down
	sigch := make(chan os.Signal, 3)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGHUP, os.Interrupt)

	// loop until signal sent
	sig := <-sigch

	log.Infow("shutdown", "requested by signal", sig)
	if creds == "" {
		wfb.Close()
		// wps.Shutdown()
	}
	if _, err := os.Stat(riscPath); err == nil {
		risc.DisableWebhook()
	}
	if config.TGRunning() {
		wtg.Shutdown()
	}

	// _ = log.Sync()

	// close database connection
	model.Disconnect()

	if c.String("https") != "none" {
		_ = wasabeehttps.Shutdown()
	}

	return nil
}
