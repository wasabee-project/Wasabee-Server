package main

import (
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"

	"github.com/op/go-logging"
	"github.com/urfave/cli"
	"github.com/wasabee-project/Wasabee-Server"
	"github.com/wasabee-project/Wasabee-Server/Firebase"
	"github.com/wasabee-project/Wasabee-Server/GroupMe"
	"github.com/wasabee-project/Wasabee-Server/RISC"
	"github.com/wasabee-project/Wasabee-Server/Telegram"
	"github.com/wasabee-project/Wasabee-Server/http"
	// "runtime/pprof"
)

var flags = []cli.Flag{
	cli.StringFlag{
		Name: "database, d", EnvVar: "DATABASE", Value: "wasabee:GoodPassword@tcp(localhost)/wasabee",
		Usage: "MySQL/MariaDB connection string. It is recommended to pass this parameter as an environment variable."},
	cli.StringFlag{
		Name: "certs", EnvVar: "CERTDIR", Value: "./certs/",
		Usage: "Directory where HTTPS certificates are stored."},
	cli.StringFlag{
		Name: "root, r", EnvVar: "ROOT_URL", Value: "https://wasabee.phtiv.com",
		Usage: "The path under which the application will be reachable from the internet."},
	cli.StringFlag{
		Name: "wordlist", EnvVar: "WORD_LIST", Value: "eff_large_wordlist.txt",
		Usage: "Word list used for random slug generation."},
	cli.StringFlag{
		Name: "log", EnvVar: "LOGFILE", Value: "logs/wasabee.log",
		Usage: "output log file."},
	cli.StringFlag{
		Name: "https", EnvVar: "HTTPS_LISTEN", Value: ":443",
		Usage: "HTTPS listen address."},
	cli.StringFlag{
		Name: "httpslog", EnvVar: "HTTPS_LOGFILE", Value: "logs/wasabee-https.log",
		Usage: "HTTPS log file."},
	cli.StringFlag{
		Name: "frontend-path, p", EnvVar: "FRONTEND_PATH", Value: "./frontend",
		Usage: "Location of the frontend files."},
	cli.StringFlag{
		Name: "googleclient", EnvVar: "GOOGLE_CLIENT_ID", Value: "UNSET",
		Usage: "Google ClientID. It is recommended to pass this parameter as an environment variable"},
	cli.StringFlag{
		Name: "googlesecret", EnvVar: "GOOGLE_CLIENT_SECRET", Value: "UNSET",
		Usage: "Google Client Secret. It is recommended to pass this parameter as an environment variable"},
	cli.StringFlag{
		Name: "sessionkey", EnvVar: "SESSION_KEY", Value: "",
		Usage: "Session Key (32 char, random). It is recommended to pass this parameter as an environment variable"},
	cli.StringFlag{
		Name: "tgkey", EnvVar: "TELEGRAM_API_KEY", Value: "",
		Usage: "Telegram API Key. It is recommended to pass this parameter as an environment variable"},
	cli.StringFlag{
		Name: "venlonekey", EnvVar: "VENLONE_API_KEY", Value: "",
		Usage: "V.enl.one API Key. It is recommended to pass this parameter as an environment variable"},
	cli.BoolFlag{
		Name: "venlonepoller", EnvVar: "VENLONE_POLLER",
		Usage: "Poll status.enl.one for RAID/JEAH location data."},
	cli.StringFlag{
		Name: "enlrockskey", EnvVar: "ENLROCKS_API_KEY", Value: "",
		Usage: "enl.rocks API Key. It is recommended to pass this parameter as an environment variable"},
	cli.StringFlag{
		Name: "enliokey", EnvVar: "ENLIO_API_KEY", Value: "",
		Usage: "enl.io API token. It is recommended to pass this parameter as an environment variable"},
	cli.StringFlag{
		Name: "gmbotkey", EnvVar: "GROUPME_ACCESS_TOKEN", Value: "",
		Usage: "GroupMe Access Token."},
	cli.BoolFlag{
		Name: "debug", EnvVar: "DEBUG",
		Usage: "Show (a lot) more output."},
	cli.BoolFlag{
		Name:  "help, h",
		Usage: "Shows this help, then exits."},
}

func main() {
	app := cli.NewApp()

	app.Name = "wasabee-server"
	app.Version = "0.6.9"
	app.Usage = "Wasabee Server"
	app.Authors = []cli.Author{
		{
			Name:  "Scot C. Bontrager",
			Email: "scot@indievisible.org",
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
	if c.Bool("help") {
		_ = cli.ShowAppHelp(c)
		return nil
	}

	if c.Bool("debug") {
		wasabee.SetLogLevel(logging.DEBUG)
	}
	if c.String("log") != "" {
		_ = wasabee.AddFileLog(c.String("log"), logging.INFO)
	}

	// Load words
	err := wasabee.LoadWordsFile(c.String("wordlist"))
	if err != nil {
		wasabee.Log.Errorf("Error loading word list from '%s': %s", c.String("wordlist"), err)
	}

	// load the UI templates
	ts, err := wasabee.TemplateConfig(c.String("frontend-path"))
	if err != nil {
		wasabee.Log.Errorf("unable to load frontend templates from %s; shutting down", c.String("frontend-path"))
		panic(err)
	}

	// Connect to database
	err = wasabee.Connect(c.String("database"))
	if err != nil {
		wasabee.Log.Errorf("Error connecting to database: %s", err)
		panic(err)
	}

	// setup V
	if c.String("venlonekey") != "" {
		wasabee.SetVEnlOne(c.String("venlonekey"))
		if c.Bool("venlonepoller") {
			go wasabee.StatusServerPoller()
		}
	}

	// setup Rocks
	if c.String("enlrockskey") != "" {
		wasabee.SetEnlRocks(c.String("enlrockskey"))
	}

	// setup enl.io
	if c.String("enliokey") != "" {
		wasabee.SetENLIO(c.String("enliokey"))
	}

	// Serve HTTPS
	if c.String("https") != "none" {
		go wasabeehttps.StartHTTP(wasabeehttps.Configuration{
			ListenHTTPS:      c.String("https"),
			FrontendPath:     c.String("frontend-path"),
			Root:             c.String("root"),
			CertDir:          c.String("certs"),
			GoogleClientID:   c.String("googleclient"),
			GoogleSecret:     c.String("googlesecret"),
			CookieSessionKey: c.String("sessionkey"),
			Logfile:          c.String("httpslog"),
			TemplateSet:      ts,
		})
	}

	riscPath := path.Join(c.String("certs"), "risc.json")
	if _, err := os.Stat(riscPath); err != nil {
		wasabee.Log.Noticef("%s does not exist, not enabling RISC", riscPath)
	} else {
		go risc.RISC(riscPath)
	}

	firebasePath := path.Join(c.String("certs"), "firebase.json")
	if _, err := os.Stat(firebasePath); err != nil {
		wasabee.Log.Noticef("%s does not exist, not enabling Firebase", firebasePath)
	} else {
		go wasabeefirebase.ServeFirebase(firebasePath)
	}

	// Serve Telegram
	if c.String("tgkey") != "" {
		go wasabeetelegram.WasabeeBot(wasabeetelegram.TGConfiguration{
			APIKey:      c.String("tgkey"),
			HookPath:    "/tg",
			TemplateSet: ts,
		})
	}

	// Serve Groupme
	if c.String("gmbotkey") != "" {
		go wasabeegm.GMbot(wasabeegm.GMConfiguration{
			AccessToken: c.String("gmbotkey"),
			TemplateSet: ts,
		})
	}

	// wait for signal to shut down
	sigch := make(chan os.Signal, 3)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGHUP, os.Interrupt)

	// loop until signal sent
	sig := <-sigch

	wasabee.Log.Info("Shutdown Requested: ", sig)
	if _, err := os.Stat(riscPath); err == nil {
		risc.DisableWebhook()
	}
	if r, _ := wasabee.TGRunning(); r {
		wasabeetelegram.Shutdown()
	}
	if c.String("https") != "none" {
		_ = wasabeehttps.Shutdown()
	}

	// close database connection
	wasabee.Disconnect()
	return nil
}
