package main

import (
	"os"
	"strings"

	"github.com/cloudkucooland/WASABI"
	"github.com/op/go-logging"
	"github.com/urfave/cli"
)

var flags = []cli.Flag{
	cli.StringFlag{
		Name: "database, d", EnvVar: "DATABASE", Value: "wasabi:GoodPassword@tcp(localhost)/wasabi",
		Usage: "MySQL/MariaDB connection string. It is recommended to pass this parameter as an environment variable."},
	cli.StringFlag{
		Name: "venlonekey", EnvVar: "VENLONE_API_KEY", Value: "",
		Usage: "V.enl.one API Key. It is recommended to pass this parameter as an environment variable"},
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

	app.Name = "WASABI-userupdate"
	app.Version = "0.0.1"
	app.Usage = "WASABI User Update"
	app.Authors = []cli.Author{
		cli.Author{
			Name:  "Scot C. Bontrager",
			Email: "scot@indievisible.org",
		},
	}
	app.Copyright = "© Scot C. Bontrager"
	app.HelpName = "wasabi-userupdate"
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

	// Connect to database
	err := WASABI.Connect(c.String("database"))
	if err != nil {
		WASABI.Log.Errorf("Error connecting to database: %s", err)
		panic(err)
	}

	// setup V
	if c.String("venlonekey") != "" {
		WASABI.SetVEnlOne(c.String("venlonekey"))
	}

	// setup Rocks
	if c.String("enlrockskey") != "" {
		WASABI.SetEnlRocks(c.String("enlrockskey"))
	}

	err = WASABI.RevalidateEveryone()
	if err != nil {
		WASABI.Log.Errorf("Revalidate Failed: %s", err)
		panic(err)
	}
	return nil
}
