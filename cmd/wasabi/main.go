package main

import (
	"os"
	"strings"

	"github.com/cloudkucooland/WASABI"
	"github.com/cloudkucooland/WASABI/Telegram"
	"github.com/cloudkucooland/WASABI/http"
	"github.com/op/go-logging"
	"github.com/urfave/cli"
)

var flags = []cli.Flag{
	cli.StringFlag{
		Name: "database, d", EnvVar: "DATABASE", Value: "wasabi:GoodPassword@tcp(localhost)/wasabi",
		Usage: "MySQL/MariaDB connection string. It is recommended to pass this parameter as an environment variable."},
	cli.StringFlag{
		Name: "certs", EnvVar: "CERTDIR", Value: "./certs/",
		Usage: "Directory where HTTPS certificates are stored."},
	cli.StringFlag{
		Name: "root, r", EnvVar: "ROOT_URL", Value: "https://wasabi.phtiv.com",
		Usage: "The path under which the application will be reachable from the internet."},
	cli.StringFlag{
		Name: "wordlist", EnvVar: "WORD_LIST", Value: "eff_large_wordlist.txt",
		Usage: "Word list used for random slug generation."},
	cli.StringFlag{
		Name: "https", EnvVar: "HTTPS_LISTEN", Value: ":443",
		Usage: "HTTPS listen address."},
	cli.StringFlag{
		Name: "httpslog", EnvVar: "HTTPS_LOGFILE", Value: "wasabi-https.log",
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
	cli.BoolFlag{
		Name: "debug", EnvVar: "DEBUG",
		Usage: "Show (a lot) more output."},
	cli.BoolFlag{
		Name:  "help, h",
		Usage: "Shows this help, then exits."},
}

func main() {
	app := cli.NewApp()

	app.Name = "WASABI"
	app.Version = "0.6.7"
	app.Usage = "WASABI Server"
	app.Authors = []cli.Author{
		cli.Author{
			Name:  "Scot C. Bontrager",
			Email: "scot@indievisible.org",
		},
	}
	app.Copyright = "© Scot C. Bontrager"
	app.HelpName = "wasabi"
	app.Flags = flags
	app.HideHelp = true
	cli.AppHelpTemplate = strings.Replace(cli.AppHelpTemplate, "GLOBAL OPTIONS:", "OPTIONS:", 1)

	app.Action = run

	app.Run(os.Args)
}

func run(c *cli.Context) error {
	if c.Bool("help") {
		cli.ShowAppHelp(c)
		return nil
	}

	if c.Bool("debug") {
		WASABI.SetLogLevel(logging.DEBUG)
	}

	// Load words
	err := WASABI.LoadWordsFile(c.String("wordlist"))
	if err != nil {
		WASABI.Log.Errorf("Error loading word list from '%s': %s", c.String("wordlist"), err)
	}

	// Connect to database
	err = WASABI.Connect(c.String("database"))
	if err != nil {
		WASABI.Log.Errorf("Error connecting to database: %s", err)
		panic(err)
	}

	// setup V
	if c.String("venlonekey") != "" {
		WASABI.SetVEnlOne(c.String("venlonekey"))
		if c.Bool("venlonepoller") {
			go WASABI.StatusServerPoller()
		}
	}

	// setup Rocks
	if c.String("enlrockskey") != "" {
		WASABI.SetEnlRocks(c.String("enlrockskey"))
	}

	// Serve HTTPS
	if c.String("https") != "none" {
		go wasabihttps.StartHTTP(wasabihttps.Configuration{
			ListenHTTPS:      c.String("https"),
			FrontendPath:     c.String("frontend-path"),
			Root:             c.String("root"),
			CertDir:          c.String("certs"),
			GoogleClientID:   c.String("googleclient"),
			GoogleSecret:     c.String("googlesecret"),
			CookieSessionKey: c.String("sessionkey"),
			Logfile:          c.String("httpslog"),
		})
	}

	// Serve Telegram
	if c.String("tgkey") != "" {
		go wasabitelegram.WASABIBot(wasabitelegram.TGConfiguration{
			APIKey:       c.String("tgkey"),
			FrontendPath: c.String("frontend-path"),
		})
	}

	// Sleep
	select {}
}
