package node

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"pnt/conf"
	"pnt/db"
	"pnt/db/model"
	"pnt/log"
	"pnt/utils"
	"strconv"
	"strings"
	"time"
)

//InitNode node init
func InitNode(con *conf.Config) error {
	nodeID := utils.UniqueID()
	createChainAccount(nodeID)
	log.GetLogHandler().Info("init node,nodeId is ", nodeID)
	ip, err := utils.GetIP()
	if err != nil {
		log.GetLogHandler().Error("Get ip-countyCode error: ", err.Error())
		return err
	}
	//log.GetLogHandler().Info("Public IP is  ", ip)
	//IP+Port
	cnataAddr := ip + ":" + strconv.Itoa(con.ControlPort)
	//国家代号，例如中国CN，美国US
	countryCode, err := utils.GetCountryCode()
	if err != nil {
		log.GetLogHandler().Error("Get ip-countyCode error: ", err.Error())
		return err
	}
	//log.GetLogHandler().Info("Country Code is ", countryCode)
	//获取国家名称，例如中国-china，日本-japan
	country, err := utils.GetCountry()
	if err != nil {
		log.GetLogHandler().Error("Get ip-countyCode error: ", err.Error())
		return err
	}
	//log.GetLogHandler().Info("Country name is ", country)
	//regionname，地区名称，例如北京-beijing
	region, err := utils.GetRegionName()
	if err != nil {
		log.GetLogHandler().Error("Get ip-countyCode error: ", err.Error())
		return err
	}
	//log.GetLogHandler().Info("region is ", region)
	//地区  国家名称+地区名称
	//region := country + "," + regionname
	rand.Seed(time.Now().Unix())
	speed := rand.Float64() + 30.0
	var providedList []string
	//server model
	if con.ServerModel && speed > 20.0 {
		/*
			//memory := GetMemory()
			memory := 10
			cpuNum := runtime.NumCPU()
			score = int(speed)*50 + int(memory)*25 + cpuNum*25
			if score > 1500 {
				level = 'S'
			} else if score > 1200 && score <= 1500 {
				level = 'A'
			} else {
				level = 'B'
			}
		*/
		providedList = append(providedList, "http")
		providedList = append(providedList, "https")
		providedList = append(providedList, "tcp")
	}

	bn := new(model.BasicNode)
	bn.Limits = model.Limits{
		MaxConn: 50,
		Speed:   600.0,
	}
	bn.Hardware = model.Hardware{
		BandWidth: speed,
		CPU:       2.5,
		GPU:       2,
		Disk:      256,
	}
	bn.NodeInfo = model.NodeInfo{
		CNATAddr:     cnataAddr,
		NodeID:       nodeID,
		VPSAddr:      ip,
		ControlPort:  con.ControlPort,
		DataTunnPort: con.TunnelPort,
		Alias:        "xxxx",
		//Region:        "China,Beijing",
		Country: country,
		Region:  region,
		//CountyCode:    "CN",
		CountryCode:   countryCode,
		WorkStatus:    1,
		Roler:         getRoler(con),
		FreshInterval: 0,
		ProvideList:   providedList,
	}
	if con.Genesis {
		bn.NodeInfo.DNATAddr = fmt.Sprintf("%s:%d", ip, bn.NodeInfo.DataTunnPort)
	}
	// TODO Roler 启动和 // nodeSort.go
	db.GetManager().BasicNodeDao().CreateTable(bn)
	db.GetManager().BasicNodeDao().AddSelfItem(bn)
	if con.ServerModel || con.Genesis {
		db.GetManager().RepeaterNodeDao().InitServerRepeater()
	}
	log.GetLogHandler().Infoln("create self node: ", *bn)
	return nil
}

func getRoler(conf *conf.Config) int {
	var source = model.CONSUMER
	if conf.Genesis || conf.ServerModel {
		source += model.PROVIDER
		// TODO 中继器判断
		source += model.REPEATERS
	}
	return source
}

func createChainAccount(uid string) {
	data := fmt.Sprintf(`{"Args":["createaccount","%s"]}`, uid)
	res, err := http.Post("http://150.109.11.142:4090/api/v1/peer/chaincode/mycc", "application/json", strings.NewReader(data))
	if err != nil {
		log.GetLogHandler().Error("createChainAccount error: ", err.Error())
	}
	body, _ := ioutil.ReadAll(res.Body)
	log.GetLogHandler().Debugf("createChainAccount Success: %s", string(body))
	res.Body.Close()
}

// import (
//     "github.com/StackExchange/wmi"
//     "fmt"
// )

// type gpuInfo struct {
//     Name string
// }

// func getGPUInfo() {

//     var gpuinfo []gpuInfo
//     err := wmi.Query("Select * from Win32_VideoController", &gpuinfo)
//     if err != nil {
//         return
//     }
//     fmt.Printf("GPU:=",gpuinfo[0].Name)
// }
