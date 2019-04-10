package main

import (
	"fmt"
	"os"
	"pnt/cmd"
	"pnt/utils"
	"strconv"
	"time"

	"github.com/urfave/cli"
)

var compileAt string

var (
	app = cli.NewApp()
	//Seednodes seednodes 填写种子节点
	Seednodes = cli.StringFlag{
		Name:   "seednode, n",
		EnvVar: "SEEDNODES",
		Usage:  "seednodes host string",
		//Value:  "127.0.0.1:8800,122.0.0.1:8899",
		Value: "150.109.11.142:38888",
	}
	//ControlPort ControlPort
	ControlPort = cli.UintFlag{
		Name:  "controlport, c",
		Usage: "control tunnel port",
		Value: 38888,
	}
	//TunnClientPort DataClientPort
	TunnClientPort = cli.UintFlag{
		Name:  "port, p",
		Usage: "data tunnel client port",
		Value: 12345,
	}
	//SortHandlerPort SortHandlerPort
	SortHandlerPort = cli.UintFlag{
		Name:  "sorthandlerport, a",
		Usage: "sort handler port",
		Value: 12398,
	}
	//ServerModel ServerModel
	ServerModel = cli.BoolFlag{
		Name:  "server, s",
		Usage: "start with server model",
	}
	//GenesisFlag GenesisFlag
	GenesisFlag = cli.BoolFlag{
		Name:  "genesis, g",
		Usage: "start as genesis model",
	}
	//DebugModel DebugModel
	DebugModel = cli.StringFlag{
		Name:  "debug",
		Usage: "Quiet start release, debug, test default debug",
		Value: "debug",
	}
	//LogModel LogModel
	LogModel = cli.IntFlag{
		Name:  "logmodel",
		Usage: "log model set, 0 to cache, 1 to file",
		Value: 1,
	}
	//CacheLines CacheLines
	CacheLines = cli.IntFlag{
		Name:  "cachelines",
		Usage: "cache lines set",
		Value: 500,
	}
	//LogFilePath LogFilePath
	LogFilePath = cli.StringFlag{
		Name:  "logpath",
		Usage: "redirect log into file",
		Value: "/tmp/kerri.log",
	}
	//APIPort ApiPort
	APIPort = cli.IntFlag{
		Name:  "apiport",
		Usage: "api server port",
		Value: 48899,
	}
)

func init() {
	app.Action = cmd.Pnt
	app.Name = "kerrigan"
	timestamp, _ := strconv.ParseInt(compileAt, 10, 64)
	app.Compiled = time.Unix(timestamp, 0)
	version := os.Getenv("QUEEN_VERSION")
	if version == "" {
		app.Version = "1.0.0"
	}
	app.Usage = "the kerrigan usage"
	app.Copyright = "Copyright 2017-2018 pujielan"
	app.Flags = []cli.Flag{
		Seednodes,
		ServerModel,
		ControlPort,
		TunnClientPort,
		GenesisFlag,
		DebugModel,
		LogModel,
		CacheLines,
		LogFilePath,
		// TimeOut,
		APIPort,
	}
	//add plugin flags
	app.Flags = utils.AddPluginFlags(app.Flags)
}

func addPlugin() {
	app.Flags = append(app.Flags)
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
