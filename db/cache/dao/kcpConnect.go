package dao

import (
	"github.com/Sirupsen/logrus"
	"github.com/xtaci/kcp-go"
	"net"
	"pnt/db/cache/funcs"
	"pnt/db/model"
)

type KcpConnectImpl struct {
	cache *funcs.LocalCache
	log   *logrus.Logger
}

//NewKcpConnectImpl new basic node impl
func NewKcpConnectImpl(l *logrus.Logger, f *funcs.LocalCache) *KcpConnectImpl {
	return &KcpConnectImpl{
		cache: f,
		log:   l,
	}
}

//CreateTable to create table
func (b *KcpConnectImpl) CreateTable(mo model.DBInterface) {
	kcpConn := mo.(*model.KcpConnect)
	b.cache.CacheTable(kcpConn.ModelName())
	b.log.Infoln("create table ", kcpConn.ModelName())
}

//AddItem add item
func (b *KcpConnectImpl) AddItem(mo model.DBInterface) {
	kcpConn := mo.(*model.KcpConnect)
	b.cache.CacheTable(kcpConn.ModelName()).NotFoundAdd(kcpConn.Addr.String(), TIMEINTERVAL, kcpConn)
	b.log.Infof("add item kcpConn %s success", kcpConn.Addr.String())
}

//DelItem delete item
func (b *KcpConnectImpl) DelItem(addr string) {
	_, err := b.cache.CacheTable(new(model.KcpConnect).ModelName()).Delete(addr)
	if err != nil {
		b.log.Errorf("Delete KcpConnect item %s error: %s", addr, err.Error())
	}
}

func (b *KcpConnectImpl) GetKcpConnByAddr(addr string) *kcp.KCP {
	if val, err := b.cache.CacheTable(new(model.KcpConnect).ModelName()).Value(addr); err == nil {
		item := val.Data().(*model.KcpConnect)
		return item.Kcp
	}
	return nil
}

func (b *KcpConnectImpl) GetUdpAddrAll() []*net.UDPAddr {
	i := 0
	tab := b.cache.CacheTable(new(model.KcpConnect).ModelName())
	data := make([]*net.UDPAddr, tab.Count())
	tab.Foreach(
		func(key interface{}, item *funcs.CacheItem) {
			v, _ := item.Data().(*model.KcpConnect)
			data[i] = v.Addr
			i++
		},
	)
	return data
}
