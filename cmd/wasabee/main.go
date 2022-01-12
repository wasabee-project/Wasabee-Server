package main

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"cloud.google.com/go/profiler"
	"google.golang.org/api/option"

	"github.com/urfave/cli"

	"github.com/wasabee-project/Wasabee-Server/Firebase"
	"github.com/wasabee-project/Wasabee-Server/RISC"
	"github.com/wasabee-project/Wasabee-Server/Telegram"
	"github.com/wasabee-project/Wasabee-Server/background"
	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/generatename"
	"github.com/wasabee-project/Wasabee-Server/http"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/rocks"
	"github.com/wasabee-project/Wasabee-Server/templates"
	"github.com/wasabee-project/Wasabee-Server/v"

	"go.uber.org/zap"
	// "golang.org/x/oauth2/google"
)

const version = "0.99.1"

var flags = []cli.Flag{
	cli.StringFlag{
		Name: "config, f", EnvVar: "CONFIG", Value: "wasabee.json",
		Usage: "Path to the config JSON file"},
	cli.BoolFlag{
		Name:  "help, h",
		Usage: "Shows this help, then exits"},
	cli.StringFlag{
		Name: "log", EnvVar: "LOGFILE", Value: "logs/wasabee.log",
		Usage: "Output log file"},
	cli.BoolFlag{
		Name: "debug, d", EnvVar: "DEBUG",
		Usage: "Log more details"},
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

	_ = app.Run(os.Args)
}

func run(cargs *cli.Context) error {
	if cargs.Bool("help") {
		_ = cli.ShowAppHelp(cargs)
		return nil
	}

	project := os.Getenv("GCP_PROJECT")
	creds := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")

	// config depends on log, so do this the hard way
	logconf := log.Configuration{
		Console:            true,
		ConsoleLevel:       zap.InfoLevel,
		FilePath:           cargs.String("log"),
		FileLevel:          zap.InfoLevel,
		GoogleCloudProject: project,
		GoogleCloudCreds:   creds,
	}
	if cargs.Bool("debug") {
		logconf.ConsoleLevel = zap.DebugLevel
	}
	log.SetupLogging(&logconf)

	// cloud profile
	if creds != "" && project != "" && cargs.Bool("debug") {
		if _, err := os.Stat(creds); err == nil {
			if err := profiler.Start(profiler.Config{
				Service:        "wasabee",
				ServiceVersion: version,
				ProjectID:      project,
			}, option.WithCredentialsFile(creds)); err != nil {
				log.Errorw("startup", "message", "unable to start profiler", "error", err)
			} else {
				log.Infow("startup", "message", "starting gcloud profiling")
			}
		}
	}

	// the main context used for all sub-services, when this is canceled, everything shuts down
	ctx, shutdown := context.WithCancel(context.Background())

	// load the config file
	conf, err := config.LoadFile(cargs.String("config"))
	if err != nil {
		log.Fatal(err)
	}

	// Load words
	if err := generatename.LoadWordsFile(conf.WordListFile); err != nil {
		log.Fatalw("startup", "message", "Error loading word list", "wordlist", conf.WordListFile, "error", err.Error())
	}

	// load the UI templates
	if err := templates.Start(conf.FrontendPath); err != nil {
		log.Fatalw("startup", "message", "unable to load frontend templates; shutting down", "path", conf.FrontendPath, "error", err.Error())
	}

	// Connect to database
	if err = model.Connect(conf.DB); err != nil {
		log.Fatalw("startup", "message", "Error connecting to database", "error", err.Error())
	}

	// start background tasks
	go background.Start(ctx)

	// start V
	go v.Start(ctx)

	// start Rocks
	go rocks.Start(ctx)

	// start firebase
	go wfb.Start(ctx)

	// Serve HTTPS -- does not use the context
	go wasabeehttps.Start()

	// RISC and Telegram should start after https
	go risc.Start(ctx)

	// Serve Telegram
	go wtg.Start(ctx)

	// everything is running. Wait for the OS to signal time to stop

	// wait for signal to shut down
	sigch := make(chan os.Signal, 3)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGHUP, os.Interrupt)

	// loop until signal sent
	sig := <-sigch
	log.Infow("shutdown", "requested by signal", sig)

	// shutdown RISC, Telegram, V, Rocks, and Firebase by canceling the context
	shutdown()

	// shutdown the http server
	if err = wasabeehttps.Shutdown(); err != nil {
		log.Error(err)
	}

	// close database connection
	model.Disconnect()

	// _ = log.Sync()
	return nil
}
