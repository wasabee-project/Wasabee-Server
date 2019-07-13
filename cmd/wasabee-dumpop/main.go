package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/wasabee-project/Wasabee-Server"
	"github.com/op/go-logging"
	"github.com/urfave/cli"
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

	app.Name = "wasabee-dumpop"
	app.Version = "0.0.1"
	app.Usage = "WASABI op exporter"
	app.Authors = []cli.Author{
		{
			Name:  "Scot C. Bontrager",
			Email: "scot@indievisible.org",
		},
	}
	app.Copyright = "© Scot C. Bontrager"
	app.HelpName = "wasabee-dumpop"
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
	opID := wasabee.OperationID(c.Args().First())

	if c.Bool("help") {
		_ = cli.ShowAppHelp(c)
		return nil
	}

	if c.Bool("debug") {
		wasabee.SetLogLevel(logging.DEBUG)
	}

	// Connect to database
	err := wasabee.Connect(c.String("database"))
	if err != nil {
		panic(err)
	}

	// do the work
	output, err := dumpop(wasabee.GoogleID(c.String("gid")), opID)
	if err != nil {
		panic(err)
	}
	fmt.Print(string(output))
	return nil
}
