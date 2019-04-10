//server 端常驻监听

package kcpt

import (
	"fmt"
	"io"
	"io/ioutil"
	"pnt/db"
	"pnt/db/boltdb/model"
	"strings"
	"sync"
	// "github.com/kavu/go_reuseport"
	"github.com/Sirupsen/logrus"
	"github.com/golang/snappy"
	"github.com/urfave/cli"
	"github.com/xtaci/kcp-go"
	"github.com/xtaci/smux"
	"net"
	"net/http"
	"pnt/log"
	"pnt/utils"
	"time"
)

//TunnServ KcpServ
type TunnServ struct {
	KCPConfig    *KCPConfig
	Block        kcp.BlockCrypt
	Log          *logrus.Logger
	servListener net.Listener
	ctx          *cli.Context
	Agent        model.Agent
}

var (
	serverInstance ServInterface
	singleInstance sync.Mutex
)

//NewTunnServ return new tunn interface
func NewTunnServ(moke *KCPMoke) ServInterface {
	ts := new(TunnServ)
	ts.KCPConfig = moke.kcpconfig
	ts.Block = moke.kcpconfig.SetCrypt()
	ts.Log = moke.Log
	ts.ctx = moke.Ctx
	node, _ := db.GetManager().BasicNodeDao().GetSelfItem()
	ts.Agent.NodeId = node.NodeInfo.NodeID
	ts.Agent.IsCustomer = 0
	ts.Agent.CreateAt = time.Now()
	serverInstance = ts
	return ts
}

//GetServerInterface GetServerInterface
func GetServerInterface() ServInterface {
	singleInstance.Lock()
	defer singleInstance.Unlock()
	if serverInstance == nil {
		log.GetLogHandler().Error("get server instance error")
	}
	return serverInstance
}

//Run run
func (s *TunnServ) Run() error {
	go s.timer()
	lis, err := kcp.ListenWithOptions(
		s.KCPConfig.ServerListen,
		s.Block,
		s.KCPConfig.DataShard,
		s.KCPConfig.ParityShard,
	)
	if err != nil {
		s.Log.Error("server listen options: ", err.Error())
		return err
	}
	s.Log.Debug("server tunnel listening on:", lis.Addr())
	//DSCP
	if err := lis.SetReadBuffer(s.KCPConfig.SockBuf); err != nil {
		s.Log.Debug("SetReadBuffer:", err.Error())
		return err
	}
	if err := lis.SetWriteBuffer(s.KCPConfig.SockBuf); err != nil {
		s.Log.Debug("SetWriteBuffer:", err.Error())
		return err
	}

	//TODO: 性能测试 火焰图
	if s.KCPConfig.Pprof {
		go http.ListenAndServe(":64444", nil)
	}

	//启动分拣器
	// if err := s.startSortListener(); err != nil {
	// 	s.Log.Errorf("start sort listener error: ", err.Error())
	// 	return err
	// }
	//20181215 11:29
	for {
		if conn, err := lis.AcceptKCP(); err == nil {
			s.Log.Debug("remote address:", conn.RemoteAddr())
			conn.SetStreamMode(true)
			conn.SetWriteDelay(false)
			conn.SetNoDelay(
				s.KCPConfig.NoDelay,
				s.KCPConfig.Interval,
				s.KCPConfig.Resend,
				s.KCPConfig.NoCongestion,
			)
			conn.SetMtu(s.KCPConfig.MTU)
			conn.SetWindowSize(s.KCPConfig.SndWnd, s.KCPConfig.RcvWnd)
			conn.SetACKNoDelay(s.KCPConfig.AckNodelay)

			s.KCPConfig.Target = utils.NewHTTPProxyPlugin(s.ctx).GetPluginAddrs()
			s.Log.Debug("target kcp listen to ", s.KCPConfig.Target)
			if s.KCPConfig.NoComp {
				go handleMux(conn, s.KCPConfig)
			} else {
				go handleMux(newCompStream(conn), s.KCPConfig)
			}
		} else {
			s.Log.Warnf("%+v", err)
		}
	}
}

type compStream struct {
	conn net.Conn
	w    *snappy.Writer
	r    *snappy.Reader
}

func (c *compStream) Read(p []byte) (n int, err error) {
	return c.r.Read(p)
}

func (c *compStream) Write(p []byte) (n int, err error) {
	n, err = c.w.Write(p)
	err = c.w.Flush()
	return n, err
}

func (c *compStream) Close() error {
	return c.conn.Close()
}

func newCompStream(conn net.Conn) *compStream {
	c := new(compStream)
	c.conn = conn
	c.w = snappy.NewBufferedWriter(conn)
	c.r = snappy.NewReader(conn)
	return c
}

func handleMux(conn io.ReadWriteCloser, config *KCPConfig) {
	// stream multiplex
	smuxConfig := smux.DefaultConfig()
	smuxConfig.MaxReceiveBuffer = config.SockBuf
	smuxConfig.KeepAliveInterval = time.Duration(config.KeepAlive) * time.Second
	mux, err := smux.Server(conn, smuxConfig)
	if err != nil {
		log.GetLogHandler().Println(err)
		return
	}
	defer mux.Close()
	for {
		p1, err := mux.AcceptStream()
		if err != nil {
			log.GetLogHandler().Println(err)
			return
		}
		p2, err := net.DialTimeout("tcp", config.Target, 5*time.Second)
		if err != nil {
			p1.Close()
			log.GetLogHandler().Println(err)
			continue
		}
		go serverhandleClient(p1, p2, config.Quiet)
	}
}

func serverhandleClient(p1, p2 io.ReadWriteCloser, quiet bool) {
	if quiet {
		log.GetLogHandler().Println("stream opened")
		defer log.GetLogHandler().Println("stream closed")
	}
	defer p1.Close()
	defer log.GetLogHandler().Info("p1 close")
	defer p2.Close()
	defer log.GetLogHandler().Info("p2 close")

	// start tunnel
	p1die := make(chan struct{})
	go func() {
		if size, err := io.Copy(p1, p2); err == nil {
			ser := GetServerInterface().(*TunnServ)
			ser.Agent.Size += size
		}
		close(p1die)
	}()

	p2die := make(chan struct{})
	go func() { io.Copy(p2, p1); close(p2die) }()

	// wait for tunnel termination
	select {
	case <-p1die:
		log.GetLogHandler().Println("p1die")
	case <-p2die:
		log.GetLogHandler().Println("p2die")
	}
}

//Conn conn
func (s *TunnServ) Conn() net.Listener {
	return s.servListener
}

func (s *TunnServ) timer() {
	for {
		time.Sleep(10 * time.Minute)
		for {
			time.Sleep(10 * time.Minute)
			data := fmt.Sprintf(`{"Args":["balance","%s", "%d"]}`, s.Agent.NodeId, s.Agent.Size)
			res, err := http.Post("http://150.109.11.142:4090/api/v1/peer/chaincode/mycc", "application/json", strings.NewReader(data))
			if err != nil || res.StatusCode != http.StatusOK {
				s.Log.Error("run /api/v1/peer/chaincode/mycc error data: %s", data)
				continue
			}
			body, err := ioutil.ReadAll(res.Body)
			if err != nil {
				s.Log.Error("run /api/v1/peer/chaincode/mycc error data: %s, body: %s", data, string(body))
				continue
			}
			Settlement(&s.Agent)
			res.Body.Close()
		}
	}
}

//ServInterface server interface
type ServInterface interface {
	Run() error
	Conn() net.Listener
	timer()
}
