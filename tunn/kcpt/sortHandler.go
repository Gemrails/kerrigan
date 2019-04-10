package kcpt

import (
	"net"
	"pnt/db/boltdb/model"
)

//StartSortListener SortHandler
func (s *TunnServ) startSortListener() error {
	//分拣器使用tcp的同端口
	l, err := net.Listen("tcp", "127.0.0.1"+s.KCPConfig.Target)
	if err != nil {
		return err
	}
	defer l.Close()
	s.Log.Debug("sort handle listen on: ", s.KCPConfig.Target)
	for {
		c, err := l.Accept()
		if err != nil {
			return err
		}
		go s.sortHandleStream(c)
	}
}

func (s *TunnServ) sortHandleStream(c net.Conn) {
	buffer := make([]byte, 1024)
	lens, err := c.Read(buffer)
	if err != nil {
		s.Log.Error("sorthandler error: ", err.Error())
	}
	s.Log.Info(lens)
	s.Log.Info(string(buffer[:lens]))

	//simple write to connection
	// c.Write([]byte("Hello from server"))
}

func Settlement(obj *model.Agent) {
	size := obj.Size
	mb := size / 1024 * 1024
	if mb <= 5 {
		return
	}
	obj.Size -= size
	model.SettlementLog(obj.NodeId, mb, obj.IsCustomer)
}
