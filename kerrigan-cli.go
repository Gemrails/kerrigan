package main

import (
	"fmt"
	"github.com/urfave/cli"
	"os"
	"pnt/client"
	"pnt/conf"
)

func main() {

	app := cli.NewApp()
	app.Name = "kerrigan-cli"
	app.Usage = "the kerrigan-cli usage"
	version := os.Getenv("QUEEN_VERSION")
	if version == "" {
		app.Version = "1.0.0"
	}

	app.Commands = client.GetClientCommands()
	app.Before = func(c *cli.Context) error {
		conf.NewConfig(c)
		return nil
	}
	err := app.Run(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
