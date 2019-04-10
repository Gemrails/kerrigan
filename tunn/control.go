package tunn

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/xtaci/kcp-go"
	"golang.org/x/crypto/pbkdf2"
	"net"
	"pnt/conf"
	"pnt/db"
	"pnt/db/model"
	plog "pnt/log"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Peer2Peer struct {
	TimeOut     int
	Seednodes   []string
	Genesis     bool
	ServerModel bool
	LocalNodeId string
	kcpConnMaps map[string]*kcp.KCP
	udpAddrMaps map[string]*net.UDPAddr
	Accepts     chan *Data
}

type ControlTunnel struct {
	UConn *net.UDPConn
	Log   *logrus.Logger
	P2P   *Peer2Peer
	block kcp.BlockCrypt
}

type NodesData struct {
	Node []*model.BasicNode `json:"node"`
	Addr string             `json:"addr"`
}

type Data struct {
	cmd  byte
	data []byte
	addr *net.UDPAddr
}

func newP2P(c *conf.Config, node *model.BasicNode) *Peer2Peer {
	p2p := new(Peer2Peer)
	p2p.Seednodes = c.Seednodes
	p2p.Genesis = c.Genesis
	p2p.TimeOut = 5
	p2p.ServerModel = c.ServerModel
	p2p.LocalNodeId = node.NodeInfo.NodeID
	p2p.Accepts = make(chan *Data, 1024)
	return p2p
}

// NewControlTunnel
func NewControlTunnel(c *conf.Config) (*ControlTunnel, error) {
	localNode, err := db.GetManager().BasicNodeDao().GetSelfItem()
	if err != nil {
		plog.GetLogHandler().Error("get basic node from db error:", err.Error())
		return nil, err
	}
	tunnel := new(ControlTunnel)
	tunnel.P2P = newP2P(c, localNode)
	tunnel.Log = plog.GetLogHandler()
	udpAddr := &net.UDPAddr{IP: net.IPv4zero, Port: localNode.NodeInfo.ControlPort}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		tunnel.Log.Errorf("listen control addr: %s err", udpAddr.String())
		return nil, err
	}
	crypt, _ := kcp.NewAESBlockCrypt(pbkdf2.Key([]byte(KEY), []byte(SALT), 4096, 32, sha1.New))
	tunnel.block = crypt
	tunnel.UConn = udpConn
	go tunnel.receiver()
	go tunnel.keepalive()
	tunnel.Log.Info("listen control addr:", udpAddr.String())
	return tunnel, nil
}

// sendNodeListTo 返回多个节点
func (c *ControlTunnel) sendNodeListTo(cmd byte, data *NodesData, addr *net.UDPAddr) error {
	for {
		if len(data.Node) <= ONCEMAXSENDNODE {
			if data, err := json.Marshal(data); err == nil {
				if _, err = c.Write(cmd, string(data[:]), addr); err != nil {
					return err
				}
			}
			break
		} else {
			data.Node = data.Node[:ONCEMAXSENDNODE]
			if data, err := json.Marshal(data); err == nil {
				if _, err = c.Write(cmd, string(data[:]), addr); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// 想全部节点发送信息
func (c *ControlTunnel) sendAll(cmd byte, data string) {
	var wg sync.WaitGroup
	for _, addr := range db.GetManager().KcpConnectDao().GetUdpAddrAll() {
		wg.Add(1)
		c.Log.Debugln("Send addr", addr.String())
		go func(addr *net.UDPAddr) {
			if _, err := c.Write(cmd, data, addr); err != nil {
				c.Log.Error("run sendAll  err ...", err.Error())
			}
			wg.Done()
		}(addr)
	}
	wg.Wait()
}

// addNode 添加节点
func (c *ControlTunnel) addNode(nodes ...*model.BasicNode) error {
	var e error = nil
	for _, node := range nodes {
		db.GetManager().BasicNodeDao().AddItem(node)
		if node.NodeInfo.Roler > 5 && node.Hardware.BandWidth > 1.0 {
			c.Log.Info("start to add bandwidth node")
			if err := db.GetManager().BandWidthProviderDao().AddOneBandProvider(node); err != nil {
				c.Log.Errorln("insert new bandwidth error: ", err.Error())
				e = err
			}
		}
	}
	return e
}

// 创建kcp链接
func (c *ControlTunnel) newKcp(convid uint32, addr *net.UDPAddr) *kcp.KCP {
	c.Log.Debugln("*********** 创建一个KCP 链接， convid->", convid, "addr -> ", addr.String())
	kcpConn := kcp.NewKCP(convid, func(buf []byte, size int) {
		if addr, err := net.ResolveUDPAddr("udp", addr.String()); err == nil {
			data := make([]byte, size)
			c.block.Encrypt(data, buf[:size])
			c.Log.Debugf("发送消息: %s, 到 -> %s", string(buf[:size]), addr.String())
			c.UConn.WriteToUDP(data, addr)
		}
	})
	db.GetManager().KcpConnectDao().AddItem(&model.KcpConnect{Kcp: kcpConn, Addr: addr})
	go func() {
		for {
			kcpConn.Update()
			time.Sleep(2 * time.Millisecond)
			if size := kcpConn.PeekSize(); size > 0 {
				buf := make([]byte, size)
				kcpConn.Recv(buf)
				c.P2P.Accepts <- &Data{cmd: buf[0], data: buf[1:], addr: addr}
			}
			if db.GetManager().KcpConnectDao().GetKcpConnByAddr(addr.String()) == nil {
				break
			}
		}
	}()
	return kcpConn
}

// 接受消息，喂给kcp
func (c *ControlTunnel) receiver() {
	for {
		res := make([]byte, MTU)
		lens, addr, err := c.UConn.ReadFromUDP(res)
		if lens == 0 || err != nil {
			c.Log.Info("lens is 0 or err is ", err)
			break
		} else if res[0] == TRACE {
			buf := make([]byte, 0, 10)
			buf = append(buf, ACK)
			c.UConn.WriteToUDP(buf, addr)
			continue
		} else if res[0] == ACK {
			continue
		} else if res[0] == DELETE {
			go c.Delete(res[1:lens], addr)
			continue
		} else if res[0] == PEATER || res[0] == PREPARE {
			db.GetManager().KcpConnectDao().DelItem(addr.String())
		}
		c.block.Decrypt(res, res)
		convid := binary.LittleEndian.Uint32(res)
		kcpConn := db.GetManager().KcpConnectDao().GetKcpConnByAddr(addr.String())
		if kcpConn == nil {
			kcpConn = c.newKcp(convid, addr)
		}
		kcpConn.Input(res, true, true)
	}
}

//Keepalive 维持打洞信息
func (c *ControlTunnel) keepalive() error {
	data := make([]byte, 0, 10)
	data = append(data, TRACE)
	for {
		time.Sleep(8 * time.Minute)
		for _, node := range db.GetManager().KcpConnectDao().GetUdpAddrAll() {
			l := strings.Split(node.String(), ":")
			if len(l) == 2 {
				p, err := strconv.Atoi(l[1])
				if err != nil {
					c.Log.Warnln("strconv port error")
					continue
				}
				for i := p - 1; i < p+2; i++ {
					addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", l[0], i))
					c.Log.Debugln("dig hoooo from: ", fmt.Sprintf("%s:%d", l[0], i))
					if err != nil {
						continue
					}
					go func(addr *net.UDPAddr) {
						if _, err := c.UConn.WriteToUDP(data, addr); err != nil {
							c.Log.Errorf("Run Keepalive To %s Error", addr.String())
						}
					}(addr)
				}
			}
		}
	}
}

// 控制隧道 统一发送数据
func (c *ControlTunnel) Write(action byte, data string, addr *net.UDPAddr) (int, error) {
	var kcpConn *kcp.KCP
	kcpConn = db.GetManager().KcpConnectDao().GetKcpConnByAddr(addr.String())
	if kcpConn == nil {
		convid := binary.LittleEndian.Uint32([]byte(addr.String()))
		kcpConn = c.newKcp(convid, addr)
	}
	buf := make([]byte, 0, len(data)+1)
	buf = append(buf, action)
	buf = append(buf, []byte(data)...)
	n := kcpConn.Send(buf)
	return n, nil
}

// Repeater 中继站获取全部节点信息
func (c *ControlTunnel) Peater() error {
	if c.P2P.Genesis {
		return nil
	}
	for _, host := range c.P2P.Seednodes {
		if CheckNodeTimeOut(host, c.P2P.TimeOut) {
			addr, err := net.ResolveUDPAddr("udp", host)
			if err != nil {
				continue
			}
			_, err = c.Write(PEATER, "PEATER", addr)
			return err
		}
	}
	return errors.New("not Seednodes use")
}

func (c *ControlTunnel) DoPeater(addr *net.UDPAddr) error {
	c.Log.Debugln("Run DoPeater")
	data := new(NodesData)
	data.Addr = addr.String()
	data.Node = db.GetManager().RepeaterNodeDao().GetList()
	return c.sendNodeListTo(REPEATER, data, addr)
}

// Repeater 添加中继站信息到BasicNodeDao
func (c *ControlTunnel) Repeater(data []byte) error {
	nodes := new(NodesData)
	err := json.Unmarshal(data, &nodes)
	if err != nil {
		return err
	}
	self, err := db.GetManager().BasicNodeDao().GetSelfItem()
	if err != nil {
		return err
	}
	self.NodeInfo.CNATAddr = nodes.Addr
	l := strings.Split(nodes.Addr, ":")
	p, _ := strconv.Atoi(l[1])
	self.NodeInfo.DNATAddr = fmt.Sprintf("%s:%d", l[0], p+1)
	for _, node := range nodes.Node {
		db.GetManager().BasicNodeDao().AddItem(node)
		db.GetManager().RepeaterNodeDao().AddOneRepeater(node.NodeInfo.NodeID)
	}
	return nil
}

// Ping Server
func (c *ControlTunnel) Ping() error {
	c.Log.Info("Run Ping")
	for nodeId, node := range db.GetManager().RepeaterNodeDao().GetItem() {
		if nodeId == c.P2P.LocalNodeId {
			continue
		}
		if CheckNodeTimeOut(node.NodeInfo.CNATAddr, c.P2P.TimeOut) {
			if addr, err := net.ResolveUDPAddr("udp", node.NodeInfo.CNATAddr); err == nil {
				c.Log.Infof("PING Server %s", addr)
				_, err = c.Write(PING, "PING", addr)
				return err
			}
		}
	}
	return errors.New("not Seednodes use")
}

//BroadcastPre 服务端接受新节点Ping 消息，向全网广播消息，为新节点打洞
func (c *ControlTunnel) BroadcastPre(addr *net.UDPAddr) error {
	c.Log.Info("Run BroadcastPre")
	c.sendAll(PREPARE, addr.String())
	return nil
}

//Prepare 节点接受到Prepare 消息，为新节点打洞
func (c *ControlTunnel) Prepare(host string) error {
	c.Log.Infof("Run Prepare To %s", host)
	l := strings.Split(host, ":")
	if len(l) == 2 {
		p, err := strconv.Atoi(l[1])
		if err != nil {
			c.Log.Warnln("strconv port error")
			return err
		}
		data := make([]byte, 0, 10)
		data = append(data, TRACE)
		for i := p - 1; i < p+2; i++ {
			addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", l[0], i))
			c.Log.Debugln("dig hoooo from: ", fmt.Sprintf("%s:%d", l[0], i))
			if err != nil {
				continue
			}
			go func(addr *net.UDPAddr) {
				if _, err := c.UConn.WriteToUDP(data, addr); err != nil {
					c.Log.Errorf("Run Prepare To %s Error", addr.String())
				}
			}(addr)
		}
	}
	return nil
}

//Pong 服务端返回当前机器的全部节点信息
func (c *ControlTunnel) Pong(addr *net.UDPAddr) error {
	c.Log.Infof("Run Pong To New Client")
	data := new(NodesData)
	data.Node = db.GetManager().BasicNodeDao().GetList()
	if bnself, err := db.GetManager().BasicNodeDao().GetSelfItem(); err == nil {
		data.Node = append(data.Node, bnself)
	}
	return c.sendNodeListTo(PONG, data, addr)
}

//DoPong DoPong
func (c *ControlTunnel) DoPong(data []byte) error {
	c.Log.Infof("Run DoPong")
	nodes := new(NodesData)
	err := json.Unmarshal(data, nodes)
	if err != nil {
		return err
	}
	return c.addNode(nodes.Node...)
}

// 节点接受到注册信息，注册新节点到自己list中
func (c *ControlTunnel) Register(data []byte) error {
	c.Log.Info("Run Register ")
	node := new(model.BasicNode)
	err := json.Unmarshal(data, node)
	if err != nil {
		return err
	}
	return c.addNode(node)
}

//BroadcastReg  新节点获取全网节点地址，广播注册新节点
func (c *ControlTunnel) BroadcastReg() error {
	c.Log.Info("Run BroadcastReg")
	basicNode, err := db.GetManager().BasicNodeDao().GetSelfItem()
	if err != nil {
		return err
	}
	rJSON, err := json.Marshal(basicNode)
	if err != nil {
		return err
	}
	data := string(rJSON[:])
	var wg sync.WaitGroup
	for _, node := range db.GetManager().BasicNodeDao().GetItem() {
		wg.Add(1)
		addr, err := net.ResolveUDPAddr("udp", node.NodeInfo.CNATAddr)
		if err != nil {
			continue
		}
		go func(addr *net.UDPAddr) {
			c.Log.Printf("send to node %s register", addr.String())
			if _, err := c.Write(REGISTER, data, addr); err != nil {
				c.Log.Errorf("Run BroadcastReg To %s Error", err.Error())
			}
			wg.Done()
		}(addr)
	}
	wg.Wait()
	return nil
}

// 广播删除节点
func (c *ControlTunnel) BroadcastDel() error {
	c.Log.Infoln("Run BroadcastDel")
	data := make([]byte, 0, 100)
	data = append(data, DELETE)
	data = append(data, []byte(c.P2P.LocalNodeId)...)
	var wg sync.WaitGroup
	for _, addr := range db.GetManager().KcpConnectDao().GetUdpAddrAll() {
		wg.Add(1)
		c.Log.Debugln("Send addr", addr.String())
		go func(addr *net.UDPAddr) {
			if _, err := c.UConn.WriteToUDP(data, addr); err != nil {
				c.Log.Error("run sendAll  err ...", err.Error())
			}
			wg.Done()
		}(addr)
	}
	wg.Wait()
	return nil
}

// 删除节点信息
func (c *ControlTunnel) Delete(nodeId []byte, addr *net.UDPAddr) error {
	c.Log.Infoln("Run Delete")
	db.GetManager().KcpConnectDao().DelItem(addr.String())
	if err := db.GetManager().BasicNodeDao().DelItem(string(nodeId[:])); err != nil {
		return err
	}
	return nil
}

// 发送消息
func (c *ControlTunnel) Data(msg string, addr *net.UDPAddr) error {
	_, err := c.Write(DATA, msg, addr)
	if err != nil {
		c.Log.Warnf("Run Data error %s ", err)
	}
	return err
}

func (c *ControlTunnel) Run() {
	defer c.UConn.Close()
	if err := c.Peater(); err != nil {
		c.Log.Error("Ping err stop control", err.Error())
		return
	}

	for {
		accept := <-c.P2P.Accepts
		switch accept.cmd {
		case PING:
			if err := c.BroadcastPre(accept.addr); err != nil {
				c.Log.Errorf("PING BroadcastPre Error %s", err.Error())
			}
			if err := c.Pong(accept.addr); err != nil {
				c.Log.Errorf("PING Pong Error %s", err.Error())
			}
		case DATA:
			//c.Data("wo shi peer1", remoteAddr)
		case REGISTER:
			if err := c.Register(accept.data); err != nil {
				c.Log.Errorf("REGISTER Register Error %s", err.Error())
			}
		case PONG:
			if err := c.DoPong(accept.data); err != nil {
				c.Log.Errorf("PONG Error %s", err.Error())
				break
			}
			if err := c.BroadcastReg(); err != nil {
				c.Log.Errorf("BroadcastReg Error %s", err.Error())
				break
			}
		case PREPARE:
			if err := c.Prepare(string(accept.data)); err != nil {
				c.Log.Errorf("PREPARE Prepare Error %s", err.Error())
				break
			}
		case DELETE:
			if err := c.Delete(accept.data, accept.addr); err != nil {
				c.Log.Errorf("DELETE Delete Error %s", err.Error())
			}
		case TRACE:
			if err := c.Data("ok", accept.addr); err != nil {
				c.Log.Errorf("TRACE Data Error %s", err.Error())
			}
		case PEATER:
			if err := c.DoPeater(accept.addr); err != nil {
				c.Log.Errorf("PEATER DoPeater Error %s", err.Error())
				break
			}
		case REPEATER:
			if err := c.Repeater(accept.data); err != nil {
				c.Log.Errorf("REPEATER Repeater %s", err.Error())
				break
			}
			if err := c.Ping(); err != nil {
				c.Log.Errorf("REPEATER Ping %s", err.Error())
				break
			}
		}
	}

}
