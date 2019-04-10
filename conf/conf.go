package conf

import (
	"errors"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"pnt/api/conf"
	"pnt/utils"
	"strings"
)

const SAVEFILE = "conf.yml"

//Config Config
type Config struct {
	Seednodes         []string     `yaml:"seednodes"`
	ShutProxy         bool         `yaml:"shut_proxy"`
	ControlPort       int          `yaml:"control_port"`
	ListenPort        int          `yaml:"listen_port"` //本地代理端口
	MissionNum        int          `yaml:"mission_num"`
	TunnelPort        int          `yaml:"data_port"`
	LinkLimies        int          `yaml:"link_limies"`
	BandWidth         float64      `yaml:"band_width"`
	Log               string       `yaml:"log"`
	Mode              string       `yaml:"mode"`
	Crypt             string       `yaml:"crypt"`
	ServerModel       bool         `yaml:"-"`
	Genesis           bool         `yaml:"genesis"`
	Quiet             bool         `yaml:"quiet"`
	LogModel          int          `yaml:"log_model"`
	LogLevel          logrus.Level `yaml:"-"`
	CacheLines        int          `yaml:"cache_lines"`
	LogPath           string       `yaml:"log_path"`
	DBModel           string       `yaml:"db_model"`
	Storage           string       `yaml:"storage"`
	utils.ProxyConfig              //plugins util
	conf.APIConfig                 // api conf
	// DataSortHandler    uint         `yaml:"sort_handler_port"` //分拣器的端口
}

var configIndex *Config

//GetKerriConfig get kerri config
func GetKerriConfig() (*Config, error) {
	if configIndex != nil {
		return configIndex, nil
	}
	return nil, errors.New("kerri index is nil")
}

//NewConfig NewConfig
func NewConfig(ctx *cli.Context) error {
	c := new(Config)
	c.LogLevel = logrus.DebugLevel
	isExist := utils.IsFileExist(SAVEFILE)
	if isExist {
		data, _ := ioutil.ReadFile(SAVEFILE)
		if err := yaml.Unmarshal(data, c); err != nil {
			fmt.Println("err config %s, system stop " + err.Error())
			return err
		}
	}

	for _, flag := range ctx.GlobalFlagNames() {
		setField(flag, ctx, c, isExist)
	}
	if err := utils.WriteToFile(SAVEFILE, c); err != nil {
		return err
	}
	configIndex = c
	return nil
}

func setField(field string, ctx *cli.Context, c *Config, confile bool) {
	if !ctx.GlobalIsSet(field) && confile {
		return
	}
	switch field {
	case "port":
		c.ListenPort = ctx.Int("port") //本地的代理监听端口
	case "controlport":
		c.ControlPort = ctx.Int("controlport")
		c.TunnelPort = c.ControlPort + 1
	case "seednode":
		seednodes := ctx.String("seednode")
		c.Seednodes = strings.Split(seednodes, ",")
	case "server":
		c.ServerModel = ctx.Bool("server")
	case "genesis":
		c.Genesis = ctx.Bool("genesis")
	case "quiet":
		c.Quiet = ctx.Bool("quiet")
	case "debug":
		val := ctx.String(field)
		c.Mode = val
		c.APIConfig.Mode = val
	case "logmodel":
		c.LogModel = ctx.Int(field)
	case "loglevel":
		c.LogLevel = logrus.Level(ctx.Int(field))
	case "cachelines":
		c.CacheLines = ctx.Int(field)
	case "logpath":
		c.LogPath = ctx.String(field)
	case "apiport":
		c.APIConfig.Port = ctx.Int(field)
	case "storage":
		c.Storage = ctx.String(field)

	}
}
