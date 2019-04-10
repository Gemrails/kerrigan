package kcpt

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/xtaci/kcp-go"
	"github.com/xtaci/smux"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"pnt/db"
	"pnt/db/boltdb/model"
	"pnt/log"
	"pnt/policy"
	"pnt/tunn"
	"strconv"
	"strings"
	"sync"
	"time"
	// "github.com/kavu/go_reuseport"
	"github.com/Sirupsen/logrus"
)

//TunnCli Kcp Client
type TunnCli struct {
	KCPConfig   *KCPConfig
	Block       kcp.BlockCrypt
	Log         *logrus.Logger
	cliListener net.Listener
	strategy    int
	pack        net.PacketConn
	Agent       model.Agent
}

var (
	clientInstance       TunnInterface
	singleClientInstance sync.Mutex
)

//NewTunnCli return new tunn interface
func NewTunnCli(moke *KCPMoke) TunnInterface {
	ts := new(TunnCli)
	ts.KCPConfig = moke.kcpconfig
	ts.Block = moke.kcpconfig.SetCrypt()
	ts.Log = moke.Log
	ts.Agent.IsCustomer = 1
	node, _ := db.GetManager().BasicNodeDao().GetSelfItem()
	ts.Agent.NodeId = node.NodeInfo.NodeID
	ts.Agent.CreateAt = time.Now()
	clientInstance = ts
	return ts
}

//GetClientInterface return client interface
func GetClientInterface() TunnInterface {
	singleClientInstance.Lock()
	defer singleClientInstance.Unlock()
	if clientInstance == nil {
		log.GetLogHandler().Error("get client instance error")
	}
	return clientInstance
}

//Run cli tunnel run
func (t *TunnCli) Run() error {
	go t.timer()
	addr, err := net.ResolveTCPAddr("tcp", t.KCPConfig.Listen)
	if err != nil {
		log.GetLogHandler().Error("Tunnel client resolveTCPAddr addr error", err.Error())
		return err
	}
	listener, err := net.Listen("tcp", addr.String())
	if err != nil {
		log.GetLogHandler().Error("Tunnel listen tcp error", err.Error())
		return err
	}
	log.GetLogHandler().Info("Tunnel client listen address on:::", addr)
	for {
		p1, err := listener.Accept()
		if err != nil {
			log.GetLogHandler().Error(err)
			return err
		}
		sessions := t.waitConn()
		if len(sessions) == 0 {
			log.GetLogHandler().Info("Have no session can be used.")
			continue
		}
		for _, ss := range sessions {
			go t.handleClient(ss, p1, t.KCPConfig.Quiet)
		}
	}
}

func (t *TunnCli) handleClient(sess *smux.Session, p1 io.ReadWriteCloser, quiet bool) {
	if !quiet {
		log.GetLogHandler().Info("stream opened")
		defer log.GetLogHandler().Info("stream closed")
	}
	defer p1.Close()
	log.GetLogHandler().Info("p1 close")
	// defer t.pack.Close()
	// log.GetLogHandler().Info("t pack close")
	p2, err := sess.OpenStream()
	if err != nil {
		return
	}
	defer p2.Close()

	// start tunnel
	p1die := make(chan struct{})
	go func() {
		if size, err := io.Copy(p1, p2); err == nil {
			cli := GetClientInterface().(*TunnCli)
			cli.Agent.Size += size
		}
		close(p1die)
	}()

	p2die := make(chan struct{})
	go func() {
		io.Copy(p2, p1)
		close(p2die)
	}()

	// wait for tunnel termination
	select {
	case <-p1die:
	case <-p2die:
	}
}

//SetStrategy set strategy in every session
func (t *TunnCli) setStrategy(s int) {
	t.strategy = s
}

func (t *TunnCli) getstrategy() {
	gy := os.Getenv(tunn.STRATEGY)
	if gy == "" {
		t.setStrategy(policy.PROXYKEEP)
	} else {
		g, _ := strconv.Atoi(gy)
		t.setStrategy(g)
	}
}

func (t *TunnCli) createSession(remoteAddr string) (*smux.Session, error) {
	t.Log.Debug("Begin to create session.")
	smuxConfig := smux.DefaultConfig()
	smuxConfig.MaxReceiveBuffer = t.KCPConfig.SockBuf
	smuxConfig.KeepAliveInterval = time.Duration(t.KCPConfig.KeepAlive) * time.Second
	kcpconn, err := kcp.DialWithOptions(
		remoteAddr,
		t.Block,
		t.KCPConfig.DataShard,
		t.KCPConfig.ParityShard,
	)
	if err != nil {
		return nil, errors.Wrap(err, "createConn(1)")
	}

	kcpconn.SetStreamMode(true)
	kcpconn.SetWriteDelay(false)
	kcpconn.SetNoDelay(
		t.KCPConfig.NoDelay,
		t.KCPConfig.Interval,
		t.KCPConfig.Resend,
		t.KCPConfig.NoCongestion,
	)
	kcpconn.SetWindowSize(t.KCPConfig.SndWnd, t.KCPConfig.RcvWnd)
	kcpconn.SetMtu(t.KCPConfig.MTU)
	kcpconn.SetACKNoDelay(t.KCPConfig.AckNodelay)
	if err := kcpconn.SetReadBuffer(t.KCPConfig.SockBuf); err != nil {
		log.GetLogHandler().Error("SetReadBuffer:", err.Error())
		return nil, errors.Wrap(err, "createConn(3)")
	}
	if err := kcpconn.SetWriteBuffer(t.KCPConfig.SockBuf); err != nil {
		log.GetLogHandler().Error("SetWriteBuffer:", err.Error())
		return nil, errors.Wrap(err, "createConn(4)")
	}

	// stream multiplex
	var session *smux.Session
	log.GetLogHandler().Info("nocomp :", t.KCPConfig.NoComp)
	if t.KCPConfig.NoComp {
		session, err = smux.Client(kcpconn, smuxConfig)
	} else {
		session, err = smux.Client(newCompStream(kcpconn), smuxConfig)
	}
	if err != nil {
		return nil, errors.Wrap(err, "createConn(2)")
	}
	log.GetLogHandler().Info("connection:", kcpconn.LocalAddr(), "->", kcpconn.RemoteAddr())
	return session, nil
}

// wait until a connection is ready
func (t *TunnCli) waitConn() []*smux.Session {
	t.getstrategy()
	ns := policy.NewSelected(t.strategy, policy.TunnProtocol(1))
	basicNodes := ns.GetSuitableNode()
	log.GetLogHandler().Info("basisNodes is :", basicNodes)
	s := make([]*smux.Session, 0, 10)
	if len(basicNodes) == 0 {
		log.GetLogHandler().Errorf("Not found any suitable node.")
		return s
	} else if len(basicNodes) > 0 {
		log.GetLogHandler().Info("get suitable item:", basicNodes)
	}
	for _, n := range basicNodes {
		log.GetLogHandler().Info("data nat addr is :", n.NodeInfo.DNATAddr, n)
		if session, err := t.createSession(n.NodeInfo.DNATAddr); err == nil {
			log.GetLogHandler().Info("success:::::")
			// log.GetLogHandler().Info(session)
			s = append(s, session)
		} else {
			log.GetLogHandler().Info("failed::::")
		}
	}
	return s
}

//Conn Conn
func (t *TunnCli) Conn() net.Listener {
	return t.cliListener
}

func (t *TunnCli) timer() {
	for {
		time.Sleep(10 * time.Minute)
		data := fmt.Sprintf(`{"Args":["payment","%s", "%d"]}`, t.Agent.NodeId, t.Agent.Size)
		res, err := http.Post("http://150.109.11.142:4090/api/v1/peer/chaincode/mycc", "application/json", strings.NewReader(data))
		if err != nil || res.StatusCode != http.StatusOK {
			t.Log.Error("run /api/v1/peer/chaincode/mycc error data: %s", data)
			continue
		}
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			t.Log.Error("run /api/v1/peer/chaincode/mycc error data: %s, body: %s", data, string(body))
			continue
		}
		Settlement(&t.Agent)
		res.Body.Close()
	}
}

//CliInterface server interface
type CliInterface interface {
	Run() error
	Conn() net.Listener
	timer()
}
