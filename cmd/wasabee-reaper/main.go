package main

import (
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/urfave/cli"
	"github.com/wasabee-project/Wasabee-Server"
	"go.uber.org/zap"
)

var flags = []cli.Flag{
	cli.StringFlag{
		Name: "database, d", EnvVar: "DATABASE", Value: "wasabee:GoodPassword@tcp(localhost)/wasabee",
		Usage: "MySQL/MariaDB connection string. It is recommended to pass this parameter as an environment variable."},
	cli.BoolFlag{
		Name: "debug", EnvVar: "DEBUG",
		Usage: "Show (a lot) more output."},
	cli.BoolFlag{
		Name:  "help, h",
		Usage: "Shows this help, then exits."},
	cli.StringFlag{
		Name: "log", EnvVar: "REAPER_LOGFILE", Value: "logs/wasabee-reaper.log",
		Usage: "output log file."},
}

func main() {
	app := cli.NewApp()

	app.Name = "wasabee-reaper"
	app.Version = "0.3.0"
	app.Usage = "WASABI Background Process"
	app.Authors = []cli.Author{
		{
			Name:  "Scot C. Bontrager",
			Email: "scot@indievisible.org",
		},
	}
	app.Copyright = "© Scot C. Bontrager"
	app.HelpName = "wasabee-reaper"
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

	logconf := wasabee.LogConfiguration{
		Console:      true,
		ConsoleLevel: zap.InfoLevel,
		FilePath:     c.String("log"),
	}
	if c.Bool("debug") {
		logconf.ConsoleLevel = zap.DebugLevel
	}
	wasabee.SetupLogging(logconf)

	// Connect to database
	err := wasabee.Connect(c.String("database"))
	if err != nil {
		wasabee.Log.Errorf("Error connecting to database: %s", err)
		return err
	}

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM, os.Interrupt)

	// this will loop until an OS signal is sent
	// Location cleanup, waypoint expiration, etc
	wasabee.BackgroundTasks(sigch)

	wasabee.Disconnect()

	return nil
}
