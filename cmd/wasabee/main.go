package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"cloud.google.com/go/profiler"
	"google.golang.org/api/option"

	"github.com/urfave/cli/v3"

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

	"go.uber.org/zap"
)

const version = "0.99.1"

var flags = []cli.Flag{
	&cli.StringFlag{
		Name:    "config",
		Aliases: []string{"f"},
		Sources: cli.EnvVars("CONFIG"),
		Value:   "wasabee.json",
		Usage:   "Path to the config JSON file",
	},
	&cli.StringFlag{
		Name:    "log",
		Sources: cli.EnvVars("LOGFILE"),
		Value:   "logs/wasabee.log",
		Usage:   "Output log file",
	},
	&cli.BoolFlag{
		Name:    "debug",
		Aliases: []string{"d"},
		Sources: cli.EnvVars("DEBUG"),
		Usage:   "Log more details",
	},
}

func main() {
	cmd := &cli.Command{
		Name:      "wasabee",
		Version:   version,
		Usage:     "Wasabee Server",
		Copyright: "© Scot C. Bontrager",
		Flags:     flags,
		Action:    run,
		Authors: []any{
			"Scot C. Bontrager <scot@wasabee.rocks>",
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		// Using fmt.Fprintf instead of log.Fatal here to allow 
		// the OS to handle the exit properly after returning from Run
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cmd *cli.Command) error {
	project := os.Getenv("GCP_PROJECT")
	creds := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")

	// The CLI context (ctx) is canceled when the command finishes or is interrupted
	appCtx, shutdown := context.WithCancel(ctx)
	defer shutdown()

	console := false
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
		console = true
	}

	logconf := log.Configuration{
		Console:            console,
		ConsoleLevel:       zap.InfoLevel,
		FilePath:           cmd.String("log"),
		FileLevel:          zap.InfoLevel,
		GoogleCloudProject: project,
		GoogleCloudCreds:   creds,
	}

	if cmd.Bool("debug") {
		logconf.ConsoleLevel = zap.DebugLevel
		if !console {
			logconf.FileLevel = zap.DebugLevel
		}
	}
	log.Start(context.Background(), &logconf)

	if creds != "" && project != "" && cmd.Bool("debug") {
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

	conf, err := config.LoadFile(cmd.String("config"))
	if err != nil {
		return fmt.Errorf("config load: %w", err)
	}

	if err := util.LoadWordsFile(conf.WordListFile); err != nil {
		return fmt.Errorf("wordlist load: %w", err)
	}

	if err := templates.Start(conf.FrontendPath); err != nil {
		return fmt.Errorf("templates start: %w", err)
	}

	if err = model.Connect(appCtx, conf.DB); err != nil {
		return fmt.Errorf("database connect: %w", err)
	}

	wtg.Init()

	// Utilizing Go 1.25 WaitGroup.Go for cleaner goroutine management
	var wg sync.WaitGroup

	wg.Go(func() { background.Start(appCtx) })
	wg.Go(func() { auth.Start(appCtx) })
	wg.Go(func() { wfb.Start(appCtx) })
	wg.Go(func() { risc.Start(appCtx) })
	wg.Go(func() { wtg.Start(appCtx) })
	wg.Go(func() { rocks.Start(appCtx) })

	// Legacy HTTP start (assume this needs modernization next to accept context)
	go wasabeehttps.Start()

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGHUP, os.Interrupt)
	
	// Wait for signal or CLI context cancellation
	select {
	case sig := <-sigch:
		log.Infow("shutdown", "requested by signal", sig)
	case <-ctx.Done():
		log.Infow("shutdown", "requested by context", ctx.Err())
	}

	// Trigger cancellation across all sub-services
	shutdown()

	// Wait for services to acknowledge appCtx cancellation and exit
	wg.Wait()

	if err = wasabeehttps.Shutdown(); err != nil {
		log.Error(err)
	}

	model.Disconnect()
	return nil
}
