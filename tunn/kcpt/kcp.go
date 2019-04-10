package kcpt

import (
	"crypto/sha1"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
	"github.com/xtaci/kcp-go"
	"golang.org/x/crypto/pbkdf2"
	"pnt/conf"
	"pnt/log"
	"sync"
)

var (
	// VERSION is injected by buildflags
	VERSION = "SELFBUILD"
	// SALT is use for pbkdf2 key expansion
	SALT = "kcp-go"
)

type (
	//KCPMoke KCP
	KCPMoke struct {
		kcpconfig *KCPConfig
		Log       *logrus.Logger
		conf      *conf.Config
		Ctx       *cli.Context
	}
	//KCPConfig KCPConfig
	KCPConfig struct {
		Listen       string `json:"listen"`
		Target       string `json:"target"`
		ServerListen string `json:"serverlisten"`
		ClientListen string `json:"clientlisten"`
		Key          string `json:"key"`
		Crypt        string `json:"crypt"`
		Mode         string `json:"mode"`
		MTU          int    `json:"mtu"`
		SndWnd       int    `json:"sndwnd"`
		RcvWnd       int    `json:"rcvwnd"`
		DataShard    int    `json:"datashard"`
		ParityShard  int    `json:"parityshard"`
		DSCP         int    `json:"dscp"`
		NoComp       bool   `json:"nocomp"`
		Conn         int    `json:"conn"`
		AckNodelay   bool   `json:"acknodelay"`
		NoDelay      int    `json:"nodelay"`
		Interval     int    `json:"interval"`
		Resend       int    `json:"resend"`
		NoCongestion int    `json:"nc"`
		SockBuf      int    `json:"sockbuf"`
		KeepAlive    int    `json:"keepalive"`
		SnmpLog      string `json:"snmplog"`
		SnmpPeriod   int    `json:"snmpperiod"`
		Pprof        bool   `json:"pprof"`
		Signal       bool   `json:"signal"` // Signal enables the signal SIGUSR1 feature.
		Quiet        bool   `json:"quiet"`
		AutoExpire   int    `json:"autoexpire"`
	}
)

// kcp default config
var (
	// DefaultKCPConfig is the default KCP config.
	// 本地端口 tunn_cli 监听  127.0.0.1:12394   control端口:38888 对外tunncli：38889(38888 + 1) 对外tunnserv: 38889(38888 + 1)
	defaultKCPConfig = &KCPConfig{
		Listen:       "127.0.0.1:1234", //客户端和服务端的监听端口 udp监听
		ServerListen: ":38889",
		ClientListen: ":38887", //本地的代理监听地址
		Target:       ":65533", //转发服务端接收的所有请求至kcphandler流处理器中进行分拣 tcp监听
		Key:          "secrect-key:HashM5:::xjsdoiuhfa920vabdsibvaiva;bc9ab9sdgfvaoyga",
		Crypt:        "aes",
		Mode:         "fast2",
		MTU:          1350,
		SndWnd:       1024,
		RcvWnd:       1024,
		Conn:         1,
		DataShard:    10,
		ParityShard:  3,
		DSCP:         0,
		NoComp:       false,
		AckNodelay:   false,
		NoDelay:      0,
		Interval:     50,
		Resend:       0,
		NoCongestion: 0,
		SockBuf:      4194304,
		KeepAlive:    10,
		SnmpLog:      "",
		SnmpPeriod:   60,
		Pprof:        false,
		Signal:       false,
		Quiet:        true,
		AutoExpire:   0,
	}
	confMux sync.Mutex
)

func initKCPConfig(conf *conf.Config, ctx *cli.Context) {
	confMux.Lock()
	defer confMux.Unlock()
	defaultKCPConfig.Listen = fmt.Sprintf(":%d", conf.ListenPort)
	defaultKCPConfig.ClientListen = fmt.Sprintf(":%d", conf.ControlPort-1)
	defaultKCPConfig.ServerListen = fmt.Sprintf(":%d", conf.ControlPort+1)
	// defaultKCPConfig.Target = fmt.Sprintf
	defaultKCPConfig.SetMode(conf)

	log.GetLogHandler().Info("Init kcp config success.")
	log.GetLogHandler().Debug("ServerListen:", defaultKCPConfig.Listen)
	log.GetLogHandler().Debug("SortHandlerListen:", defaultKCPConfig.Target)
	log.GetLogHandler().Debug("ClientListen:", defaultKCPConfig.ClientListen)
	log.GetLogHandler().Debug("encryption:", defaultKCPConfig.Crypt)
	log.GetLogHandler().Debug("nodelay parameters:", defaultKCPConfig.NoDelay, defaultKCPConfig.Interval, defaultKCPConfig.Resend, defaultKCPConfig.NoCongestion)
	log.GetLogHandler().Debug("sndwnd:", defaultKCPConfig.SndWnd, "rcvwnd:", defaultKCPConfig.RcvWnd)
	log.GetLogHandler().Debug("compression:", !defaultKCPConfig.NoComp)
	log.GetLogHandler().Debug("mtu:", defaultKCPConfig.MTU)
	log.GetLogHandler().Debug("datashard:", defaultKCPConfig.DataShard, "parityshard:", defaultKCPConfig.ParityShard)
	log.GetLogHandler().Debug("acknodelay:", defaultKCPConfig.AckNodelay)
	log.GetLogHandler().Debug("dscp:", defaultKCPConfig.DSCP)
	log.GetLogHandler().Debug("sockbuf:", defaultKCPConfig.SockBuf)
	log.GetLogHandler().Debug("keepalive:", defaultKCPConfig.KeepAlive)
	log.GetLogHandler().Debug("snmpperiod:", defaultKCPConfig.SnmpPeriod)
	log.GetLogHandler().Debug("pprof:", defaultKCPConfig.Pprof)
	log.GetLogHandler().Debug("quiet:", defaultKCPConfig.Quiet)
	return
}

//NewKCPMoke new  kcp instance
func NewKCPMoke(conf *conf.Config, ctx *cli.Context) *KCPMoke {
	initKCPConfig(conf, ctx)
	return &KCPMoke{
		Log:       log.GetLogHandler(),
		kcpconfig: defaultKCPConfig,
		Ctx:       ctx,
	}
}

//GetKCPConfigInstance get kcp config instance
func GetKCPConfigInstance() *KCPConfig {
	return defaultKCPConfig
}

//SetMode SetMode
func (c *KCPConfig) SetMode(conf *conf.Config) {
	if conf.Mode != "" {
		c.Mode = conf.Mode
	}
	log.GetLogHandler().Info("set kcp mode :", c.Mode)
}

//SetCrypt set crypt
func (c *KCPConfig) SetCrypt() kcp.BlockCrypt {
	switch c.Mode {
	case "normal":
		c.NoDelay, c.Interval, c.Resend, c.NoCongestion = 0, 40, 2, 1
	case "fast":
		c.NoDelay, c.Interval, c.Resend, c.NoCongestion = 0, 30, 2, 1
	case "fast2":
		c.NoDelay, c.Interval, c.Resend, c.NoCongestion = 1, 20, 2, 1
	case "fast3":
		c.NoDelay, c.Interval, c.Resend, c.NoCongestion = 1, 10, 2, 1
	}

	// log.Println("version:", VERSION)
	log.GetLogHandler().Info("initiating key derivation")
	pass := pbkdf2.Key([]byte(c.Key), []byte(SALT), 4096, 32, sha1.New)
	var block kcp.BlockCrypt
	switch c.Crypt {
	case "sm4":
		block, _ = kcp.NewSM4BlockCrypt(pass[:16])
	case "tea":
		block, _ = kcp.NewTEABlockCrypt(pass[:16])
	case "xor":
		block, _ = kcp.NewSimpleXORBlockCrypt(pass)
	case "none":
		block, _ = kcp.NewNoneBlockCrypt(pass)
	case "aes-128":
		block, _ = kcp.NewAESBlockCrypt(pass[:16])
	case "aes-192":
		block, _ = kcp.NewAESBlockCrypt(pass[:24])
	case "blowfish":
		block, _ = kcp.NewBlowfishBlockCrypt(pass)
	case "twofish":
		block, _ = kcp.NewTwofishBlockCrypt(pass)
	case "cast5":
		block, _ = kcp.NewCast5BlockCrypt(pass[:16])
	case "3des":
		block, _ = kcp.NewTripleDESBlockCrypt(pass[:24])
	case "xtea":
		block, _ = kcp.NewXTEABlockCrypt(pass[:16])
	case "salsa20":
		block, _ = kcp.NewSalsa20BlockCrypt(pass)
	default:
		c.Crypt = "aes"
		block, _ = kcp.NewAESBlockCrypt(pass)
	}
	return block
}
