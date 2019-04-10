package utils

import (
	"github.com/urfave/cli"
)

//PluginHandlerInterface plugin handler interface
type PluginHandlerInterface interface {
	GetPluginAddrs() string
	TroyRun()
	TroyExit()
	GetPluginProcotol() string
}

//AddPluginFlags AddPluginFlags
func AddPluginFlags(flags []cli.Flag) []cli.Flag {
	// if you have any plugins, add it in this method
	flags = append(flags, AddHTTPProxyFlags()...)
	return flags
}

//ToryPluginRun ToryPluginRun
//after registor plugins you need to set proxy run function in this method
func ToryPluginRun(ctx *cli.Context) {
	funcs := []func(){
		//your new func in here
		NewHTTPProxyPlugin(ctx).TroyRun,
	}

	for _, fc := range funcs {
		go fc()
	}
}

//ToryPluginExit ToryPluginExit
func ToryPluginExit(ctx *cli.Context) {
	funcs := []func(){
		NewHTTPProxyPlugin(ctx).TroyExit,
	}
	for _, fc := range funcs {
		go fc()
	}
}
