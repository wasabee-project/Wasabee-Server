package main

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"cloud.google.com/go/profiler"
	"google.golang.org/api/option"

	"github.com/urfave/cli"

	"github.com/wasabee-project/Wasabee-Server/Firebase"
	"github.com/wasabee-project/Wasabee-Server/RISC"
	"github.com/wasabee-project/Wasabee-Server/Telegram"
	"github.com/wasabee-project/Wasabee-Server/auth"
	"github.com/wasabee-project/Wasabee-Server/background"
	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/http"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/rocks"
	"github.com/wasabee-project/Wasabee-Server/templates"
	"github.com/wasabee-project/Wasabee-Server/util"
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

	// the main context used for all sub-services, when this is canceled, everything shuts down
	ctx, shutdown := context.WithCancel(context.Background())

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
	log.Start(context.Background(), &logconf)

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

	// load the config file
	conf, err := config.LoadFile(cargs.String("config"))
	if err != nil {
		log.Fatal(err)
	}

	// Load words
	if err := util.LoadWordsFile(conf.WordListFile); err != nil {
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

	// the waitgroup which must be completed before shutting down
	var wg sync.WaitGroup

	// start background tasks
	wg.Add(1)
	go func(ctx context.Context) {
		defer wg.Done()
		background.Start(ctx)
	}(ctx)

	// start authorization
	wg.Add(1)
	go func(ctx context.Context) {
		defer wg.Done()
		auth.Start(ctx)
	}(ctx)

	// start firebase
	wg.Add(1)
	go func(ctx context.Context) {
		defer wg.Done()
		wfb.Start(ctx)
	}(ctx)

	// Serve HTTPS -- does not use the context
	go wasabeehttps.Start()

	// start risc
	wg.Add(1)
	go func(ctx context.Context) {
		defer wg.Done()
		risc.Start(ctx)
	}(ctx)

	// start Telegram
	wg.Add(1)
	go func(ctx context.Context) {
		defer wg.Done()
		wtg.Start(ctx)
	}(ctx)

	// start V
	wg.Add(1)
	go func(ctx context.Context) {
		defer wg.Done()
		v.Start(ctx)
	}(ctx)

	// start Rocks
	wg.Add(1)
	go func(ctx context.Context) {
		defer wg.Done()
		rocks.Start(ctx)
	}(ctx)

	// everything is running. Wait for the OS to signal time to stop
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGHUP, os.Interrupt)
	sig := <-sigch
	log.Infow("shutdown", "requested by signal", sig)

	// shutdown RISC, Telegram, V, Rocks, and Firebase by canceling the context
	shutdown()

	// wait for things to complete their cleanup tasks
	wg.Wait()

	// shutdown the http server
	if err = wasabeehttps.Shutdown(); err != nil {
		log.Error(err)
	}

	// close database connection
	model.Disconnect()
	return nil
}
