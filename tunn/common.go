package tunn

import (
	"net"
	"time"
)

/*
import (
	"errors"
	"log"
	"pnt/conf"
)

//Peer2Peer Peer2Peer
type Peer2Peer struct {
	Seednodes   []string
	TimeOut     int
	OuterAddr   string
	UseInternal bool
	Err         chan error
	P2PHost		string
}

func NewP2P(c *conf.Config, errChan chan error) *Peer2Peer {
	p := new(Peer2Peer)
	p.TimeOut = c.TimeOut
	p.UseInternal = c.P2PInternal
	p.Seednodes = c.Seednodes
	p.P2PHost = c.P2PHost
	p.Err = errChan
	return p
}

//P2P P2P
var P2P *Peer2Peer

//GetP2PInstance GetP2PInstance
func GetP2PInstance() (*Peer2Peer, error){
	if P2P == nil {
		log.Println("p2p instance is nil")
		return nil, errors.New("p2p instance is nil")
	}
    return P2P, nil
}

//SetP2PInstance SetP2PInstance
func SetP2PInstance(c *conf.Config, errChan chan error) {
	P2P = newP2P(c, errChan)
}
*/

//CheckNodeTimeOut CheckNodeTimeOut
func CheckNodeTimeOut(targetIP string, timeOuts int) bool {
	udpChan := make(chan bool)
	defer close(udpChan)
	go func() {
		if addr, err := net.ResolveUDPAddr("udp", targetIP); err == nil {
			conn, err := net.DialUDP("udp", nil, addr)
			defer conn.Close()
			conn.SetReadDeadline(time.Now().Add(time.Duration(timeOuts-1) * time.Second))
			data := make([]byte, 0, 10)
			data = append(data, TRACE)
			conn.Write(data)
			if _, _, err = conn.ReadFrom(data); err != nil {
				udpChan <- false
			} else {
				udpChan <- true
			}
		} else {
			udpChan <- false
		}

	}()
	select {
	case <-time.After(time.Second * time.Duration(timeOuts)):
		return false
	case b := <-udpChan:
		return b
	}
}
