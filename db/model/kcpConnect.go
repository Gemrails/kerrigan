package model

import (
	"github.com/xtaci/kcp-go"
	"net"
)

//ModelName return model name
func (p *KcpConnect) ModelName() string {
	return "kcp_connect"
}

type KcpConnect struct {
	Kcp  *kcp.KCP
	Addr *net.UDPAddr
}
