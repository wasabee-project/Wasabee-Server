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
		Name: "venlonekey", EnvVar: "VENLONE_API_KEY", Value: "",
		Usage: "V.enl.one API Key. It is recommended to pass this parameter as an environment variable"},
	cli.StringFlag{
		Name: "enlrockskey", EnvVar: "ENLROCKS_API_KEY", Value: "",
		Usage: "enl.rocks API Key. It is recommended to pass this parameter as an environment variable"},
	cli.StringFlag{
		Name: "enlrockscommurl", EnvVar: "ENLROCKS_COMM_URL", Value: "",
		Usage: "enl.rocks Community API URL. Defaults to the enl.rocks well-known URL"},
	cli.StringFlag{
		Name: "enlrocksstatusurl", EnvVar: "ENLROCKS_STATUS_URL", Value: "",
		Usage: "enl.rocks Status API URL. Defaults to the enl.rocks well-known URL"},
	cli.BoolFlag{
		Name: "debug", EnvVar: "DEBUG",
		Usage: "Show (a lot) more output."},
	cli.BoolFlag{
		Name:  "help, h",
		Usage: "Shows this help, then exits."},
}

func main() {
	app := cli.NewApp()

	app.Name = "WASABI-userupdate"
	app.Version = "0.0.1"
	app.Usage = "WASABI User Update"
	app.Authors = []cli.Author{
		{
			Name:  "Scot C. Bontrager",
			Email: "scot@indievisible.org",
		},
	}
	app.Copyright = "© Scot C. Bontrager"
	app.HelpName = "wasabee-userupdate"
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
	}
	if c.Bool("debug") {
		logconf.ConsoleLevel = zap.DebugLevel
	}
	wasabee.SetupLogging(logconf)

	// Connect to database
	err := wasabee.Connect(c.String("database"))
	if err != nil {
		wasabee.Log.Errorf("Error connecting to database: %s", err)
		panic(err)
	}

	// setup V
	if c.String("venlonekey") != "" {
		wasabee.SetVEnlOne(wasabee.Vconfig{
			APIKey: c.String("venlonekey"),
			// XXX add URLS if set
		})
	}

	// setup Rocks
	if c.String("enlrockskey") != "" {
		wasabee.SetEnlRocks(wasabee.Rocksconfig{
			APIKey:            c.String("enlrockskey"),
			CommunityEndpoint: c.String("enlrockscommurl"),
			StatusEndpoint:    c.String("enlrocksstatusurl"),
		})
	}

	err = wasabee.RevalidateEveryone()
	if err != nil {
		wasabee.Log.Errorf("Revalidate Failed: %s", err)
		panic(err)
	}
	return nil
}
