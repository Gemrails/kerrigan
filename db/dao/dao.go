package dao

import (
	"github.com/xtaci/kcp-go"
	"net"
	"pnt/db/model"
)

//Dao Dao
type Dao interface {
	CreateTable(model.DBInterface)
	AddItem(model.DBInterface)
	UpdateItem(model.DBInterface) error
	DelItem(nodeID string) error
	GetItem(nodeID ...string) map[string]*model.BasicNode
}

//BasicNodeDao BasicNodeInfoDao
type BasicNodeDao interface {
	Dao
	//deal self node, search key "selfnode"
	AddSelfItem(model.DBInterface)
	GetSelfItem() (*model.BasicNode, error)
	GetList(nodeID ...string) []*model.BasicNode
	UpdateSelfItem(model.DBInterface) error
}

//RepeaterNodeDao RepeaterNodeDao
type RepeaterNodeDao interface {
	Dao
	AddOneRepeater(nodeID string) error
	GetList(nodeID ...string) []*model.BasicNode
	InitServerRepeater() error
}

//BandWidthProviderDao provider sort by bandwidth
type BandWidthProviderDao interface {
	Dao
	AddOneBandProvider(bn *model.BasicNode) error
	GetOneSuitableProvider() map[string]*model.BasicNode
}

type KcpConnectDao interface {
	AddItem(model.DBInterface)
	DelItem(addr string)
	GetKcpConnByAddr(addr string) *kcp.KCP
	GetUdpAddrAll() []*net.UDPAddr
}

//CPUProviderDao cpu provider
type CPUProviderDao interface {
	Dao
}

//GPUProviderDao gpu provider
type GPUProviderDao interface {
	Dao
}

//StoreProviderDao store provider
type StoreProviderDao interface {
	Dao
}
