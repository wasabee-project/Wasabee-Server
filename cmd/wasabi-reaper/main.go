package main

import (
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/cloudkucooland/WASABI"
	"github.com/op/go-logging"
	"github.com/urfave/cli"
)

var flags = []cli.Flag{
	cli.StringFlag{
		Name: "database, d", EnvVar: "DATABASE", Value: "wasabi:GoodPassword@tcp(localhost)/wasabi",
		Usage: "MySQL/MariaDB connection string. It is recommended to pass this parameter as an environment variable."},
	cli.BoolFlag{
		Name: "debug", EnvVar: "DEBUG",
		Usage: "Show (a lot) more output."},
	cli.BoolFlag{
		Name:  "help, h",
		Usage: "Shows this help, then exits."},
	cli.StringFlag{
		Name: "log", EnvVar: "REAPER_LOGFILE", Value: "logs/wasabi-reaper.log",
		Usage: "output log file."},
}

func main() {
	app := cli.NewApp()

	app.Name = "wasabi-reaper"
	app.Version = "0.3.0"
	app.Usage = "WASABI Background Process"
	app.Authors = []cli.Author{
		{
			Name:  "Scot C. Bontrager",
			Email: "scot@indievisible.org",
		},
	}
	app.Copyright = "© Scot C. Bontrager"
	app.HelpName = "wasabi-reaper"
	app.Flags = flags
	app.HideHelp = true
	cli.AppHelpTemplate = strings.Replace(cli.AppHelpTemplate, "GLOBAL OPTIONS:", "OPTIONS:", 1)

	app.Action = run

	_ = app.Run(os.Args)
}

func run(c *cli.Context) error {
	if c.Bool("help") {
		_ = cli.ShowAppHelp(c)
		return nil
	}

	if c.Bool("debug") {
		wasabi.SetLogLevel(logging.DEBUG)
	}
	if c.String("log") != "" {
		_ = wasabi.AddFileLog(c.String("log"), logging.INFO)
	}

	// Connect to database
	err := wasabi.Connect(c.String("database"))
	if err != nil {
		wasabi.Log.Errorf("Error connecting to database: %s", err)
		return err
	}

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM, os.Interrupt)

	// this will loop until an OS signal is sent
	// Location cleanup, waypoint expiration, etc
	wasabi.BackgroundTasks(sigch)

	wasabi.Disconnect()

	return nil
}
