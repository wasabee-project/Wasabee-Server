package main

import (
	"os"
	"strings"

	"github.com/urfave/cli"
	"github.com/wasabee-project/Wasabee-Server"
	"go.uber.org/zap"
)

var flags = []cli.Flag{
	cli.StringFlag{
		Name: "database, d", EnvVar: "DATABASE", Value: "wasabee:GoodPassword@tcp(localhost)/wasabee",
		Usage: "MySQL/MariaDB connection string. It is recommended to pass this parameter as an environment variable."},
	cli.StringFlag{
		Name: "gid, g", Value: "",
		Usage: "The GID of the agent who will own this op"},
	cli.BoolFlag{
		Name: "debug", EnvVar: "DEBUG",
		Usage: "Show (a lot) more output."},
	cli.BoolFlag{
		Name:  "help, h",
		Usage: "Shows this help, then exits."},
}

func main() {
	app := cli.NewApp()

	app.Name = "wasabee-loadop"
	app.Version = "0.0.1"
	app.Usage = "WASABI op importer"
	app.Authors = []cli.Author{
		{
			Name:  "Scot C. Bontrager",
			Email: "scot@indievisible.org",
		},
	}
	app.Copyright = "© Scot C. Bontrager"
	app.HelpName = "wasabee-importop"
	app.Flags = flags
	app.HideHelp = true
	cli.AppHelpTemplate = strings.Replace(cli.AppHelpTemplate, "GLOBAL OPTIONS:", "OPTIONS:", 1)

	app.Action = run

	_ = app.Run(os.Args)
}

func run(c *cli.Context) error {
	if c.Args().First() == "" {
		_ = cli.ShowAppHelp(c)
		return nil
	}
	logconf := wasabee.LogConfiguration{
		Console:      true,
		ConsoleLevel: zap.InfoLevel,
	}
	if c.Bool("debug") {
		logconf.ConsoleLevel = zap.DebugLevel
	}
	wasabee.SetupLogging(logconf)

	loadfile := c.Args().First()

	if c.Bool("help") {
		_ = cli.ShowAppHelp(c)
		return nil
	}

	// Connect to database
	err := wasabee.Connect(c.String("database"))
	if err != nil {
		panic(err)
	}

	// do the work
	err = importop(c.String("gid"), loadfile)
	if err != nil {
		panic(err)
	}
	return nil
}
