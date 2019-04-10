package kcpt

import (
	"net"
)

//TunnInterface server interface
type TunnInterface interface {
	Run() error
	Conn() net.Listener
}
