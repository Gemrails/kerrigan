package tunn

import (
	"net"
)

//ControlInterface ControlInterface
type ControlInterface interface {
	Peater() error
	Repeater(data []byte) error
	Ping() error
	BroadcastPre(addr *net.UDPAddr) error
	Pong(addr *net.UDPAddr) error
	BroadcastReg() error
	Register(jsonNode []byte) error
	Prepare(host string) error
	Data(msg string, addr *net.UDPAddr) error
	Delete(nodeID string) error
	BroadcastDel() error
	Run()
}

//DataTunnInterface DataInterface
type DataTunnInterface interface {
	Action()
}

const (
	MTU             = 1350
	KEY             = "kerrigan."
	SALT            = "e6d859e7e.c18df398"
	STRATEGY        = "STRATEGY"
	ONCEMAXSENDNODE = 1000
	//PING new_node --> seednodes
	PING byte = byte(iota)
	//PONG seednode --> new_node
	PONG
	//PREPARE repeater --> all_nodes
	PREPARE
	DATA
	//REGISTER new_node --> all_nodes
	REGISTER
	//DELETE a shutdown node --> all_nodes
	DELETE
	TRACE
	PEATER
	REPEATER
	KEEPALIVE
	ACK
)
