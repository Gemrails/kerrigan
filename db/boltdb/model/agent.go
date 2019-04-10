package model

import (
	"pnt/db/boltdb"
	"time"
)

type Agent struct {
	ID           int       `storm:"id,increment"`
	NodeId       string    `storm:"index"` // 本地NodeId
	Settlement   uint8     `storm:"index"` // 结算 0-未结算 1-结算
	IsCustomer   uint8     `storm:"index"` // 1-消费者 0-服务者
	Size         int64     // 代理请求数据大小
	CreateAt     time.Time // 任务时间
	SettlementAt time.Time // 结算时间
}

func (agent *Agent) Name() string {
	return "agent"
}

//Settlement 结算账单
func SettlementLog(nodeId string, size int64, customer uint8) {
	agent := new(Agent)
	agent.SettlementAt = time.Now()
	agent.Size = size
	agent.Settlement = 1
	agent.NodeId = nodeId
	agent.IsCustomer = customer
	_ = boltdb.GetStorage().Save(agent)
}
