package model

//Limits Limits
type Limits struct {
	MaxConn uint32  `json:"max_connection"` // 最大连接数
	Speed   float64 `json:"speed"`          // 网络速度
}

//Hardware Hardware
type Hardware struct {
	BandWidth float64     `json:"bandwidth"` // 带宽
	CPU       interface{} `json:"cpu"`       // cpu
	GPU       interface{} `json:"GPU"`       // gpu
	Disk      interface{} `json:"disk"`      // 硬盘
}

//NodeInfo NodeInfo
type NodeInfo struct {
	NodeID        string   `json:"node_id"`        // 唯一id
	Alias         string   `json:"-"`              // 别名
	VPSAddr       string   `json:"vps_addr"`       // 公网地址
	CNATAddr      string   `json:"c_nat_addr"`     // 控制隧道地址
	DNATAddr      string   `json:"d_net_addr"`     // 数据隧洞地址
	Region        string   `json:"region"`         // 地区
	WorkStatus    int      `json:"work_status"`    // 工作状态
	Roler         int      `json:"roler"`          // 角色值和
	ControlPort   int      `json:"-"`              // 控制隧道端口
	DataTunnPort  int      `json:"-"`              // 数据隧道端口
	FreshInterval int64    `json:"fresh_interval"` // 心跳更新时间间隔
	ProvideList   []string `json:"provide_list"`   // 节点的插件功能
	Country       string   `json:"country"`
	CountryCode    string   `json:"countryCode"`
}

//BasicNode BasicNode
type BasicNode struct {
	NodeInfo NodeInfo `json:"node_info"`
	Hardware Hardware `json:"hardware"`
	Limits   Limits   `json:"limits"`
}

//NewBasicNode return basic node
func NewBasicNode() *BasicNode {
	return &BasicNode{
		Limits: Limits{
			MaxConn: 10240,
			Speed:   1000,
		},
		NodeInfo: NodeInfo{
			NodeID: "self_node_key_id",
		},
	}
}

//ModelName return model name
func (b *BasicNode) ModelName() string {
	return "basic_node"
}
