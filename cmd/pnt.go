package cmd

import (
	"fmt"
	"github.com/urfave/cli"
	"os"
	"os/signal"
	"pnt/api"
	"pnt/conf"
	"pnt/db"
	"pnt/db/boltdb"
	"pnt/log"
	"pnt/node"
	"pnt/tunn"
	"pnt/tunn/kcpt"
	"pnt/utils"
	"syscall"
)

//Pnt Pnt
func Pnt(ctx *cli.Context) {
	// init all config
	if err := conf.NewConfig(ctx); err != nil {
		fmt.Printf("INIT config Error %s", err.Error())
		return
	}
	c, err := conf.GetKerriConfig()
	if err != nil {
		fmt.Printf("Get conf error %s", err.Error())
		return
	}
	// init Pnt
	if err := initPnt(c); err != nil {
		return
	}
	// run pnt
	if err := Run(ctx, c); err != nil {
		fmt.Println("run pnt proxy error.", err.Error())
		return
	}
}

func initPnt(c *conf.Config) error {
	log.InitLogger(c)
	if err := db.CreateManager(c); err != nil {
		log.GetLogHandler().Errorln("create db manager error: ", err.Error())
		return err
	}
	return node.InitNode(c)
}

//Run Pnt
func Run(ctx *cli.Context, c *conf.Config) error {
	errChan := make(chan error)
	if err := boltdb.NewStorage(c.Storage); err != nil {
		return err
	}
	controlTunnel, err := tunn.NewControlTunnel(c)
	if err != nil {
		log.GetLogHandler().Error(err.Error())
		return err
	}
	go controlTunnel.Run()
	//TODO: init kcp, kcp-server, kcp-client
	moke := kcpt.NewKCPMoke(c, ctx)
	ci := kcpt.NewTunnCli(moke)
	go ci.Run()
	if c.Genesis || c.ServerModel {
		si := kcpt.NewTunnServ(moke)
		//TODO: chan error in Run()
		go si.Run()
	}
	//TODO: 初始化完成之后 启动插件
	utils.ToryPluginRun(ctx)

	//TODO: 启动顺序问题
	// run api
	go api.Run(c)
	term := make(chan os.Signal, 1)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)
	select {
	case s := <-term:
		controlTunnel.BroadcastDel()
		fmt.Printf("Got exit signal: %s...\nBye\n", s.String())
	case err := <-errChan:
		fmt.Printf("Append error: %s\n", err.Error())
		// case warn := <-warnChan:
		// 	fmt.Printf("warning: %s\n", warn.Error())
	}

	return nil
}
