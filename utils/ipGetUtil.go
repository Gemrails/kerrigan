package utils
import (
    "bytes"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "net"
    "net/http"
    "os"
    "strconv"
    "strings"
    "errors"
)
type TaobaoIPInfo struct{
    Code int `json:"code"`
    Data TaoBaoIP  `json:"data"`
}
type TaoBaoIP struct {
	IP        string `json:"ip"`
    Country   string `json:"country"`
	Area      string `json:"area"`
	Region    string `json:"region"`
	City      string `json:"city"`
	Isp       string `json:"isp"`
	CountryId string `json:"country_id"`
    AreaId    string `json:"area_id"`
    RegionId  string `json:"region_id"`
    CityId    string `json:"city_id"`
}
type IPAPI struct {
    As           string  `json:"as"`
    City         string  `json:"city"`
    Country      string  `json:"country"`
    CountryCode  string  `json:"countryCode"`
    Isp          string  `json:"isp"`
    Latitude     float64 `json:"lat"`
    Longtitude   float64 `json:"lon"`
    Org          string  `json:"org"`
    IP           string  `json:"query"`
    Region       string  `json:"region"`
    RegionName   string  `json:"regionName"`
    Status       string  `json:"status"`
    TimeZone     string  `json:"timezone"`
    Zip          string  `json:"zip"`
}

var(
    IPapiURL      =   "http://ip-api.com/json/"
    TaobooURL     =   "http://ip.taobao.com/service/getIpInfo.php?ip="
	//TaobooURL     =   "http://ip.taobao.com/service/getIpInfo.php"
    ExternalIpURL =   "http://myexternalip.com/raw"
    GlobalIPInfoByIPapi *IPAPI 				//global information ,get IP's info by "http://ip-api.com/json/"
	GlobalIPInfoByTaoBao *TaobaoIPInfo 		//global information ,get IP's info by "http://ip.taobao.com/service/getIpInfo.php"
)
func IPapi() (*IPAPI,error){
    //for example：
    //url := "http://ip-api.com/json/65.188.186.31?lang=zh-CN"
    //url := "http://ip-api.com/json/?lang=zh-CN"
    //url := "http://ip-api.com/json/"
    resp,err := http.Get(IPapiURL)
    //resp,err := http.Get(url)
    if err != nil{
        return nil,err
    }
    defer resp.Body.Close()
    out,err := ioutil.ReadAll(resp.Body)
    if err != nil{
        return nil,err
    }
    var result IPAPI
    if err := json.Unmarshal(out, &result); err != nil {
        return nil,err
    }
    return &result,err

}
func GetInit() error{
	ip,err :=GetIPapiInfo()
	if ip == nil {
		ip,err := GetTaobaoIpInfo()
		if ip == nil {
			return err
		}
		return err
	}
	return err

}
func GetIP()(string ,error){
	//init
	var err error
	if GlobalIPInfoByIPapi == nil{
		if GlobalIPInfoByTaoBao == nil{
			err = GetInit()
			if err != nil{
				return "127.0.0.1",err
			}
		}
	}
	//get IP for return data
	if GlobalIPInfoByIPapi != nil{
		return GlobalIPInfoByIPapi.IP,nil
	}else{
		if GlobalIPInfoByTaoBao != nil{
			return GlobalIPInfoByTaoBao.Data.IP,nil
		}else{
			return "127.0.0.1",nil
		}
	}
}
func GetCountryCode()(string ,error){
	//init
	var err error
	if GlobalIPInfoByIPapi == nil{
		if GlobalIPInfoByTaoBao == nil{
			err = GetInit()
			if err != nil{
				return "",err
			}
		}
	}
	//get IP for return data
	if GlobalIPInfoByIPapi != nil{
		return GlobalIPInfoByIPapi.CountryCode,nil
	}else{
		if GlobalIPInfoByTaoBao != nil{
			return GlobalIPInfoByTaoBao.Data.CountryId,nil
		}else{
			return "None",nil
		}
	}
}
func GetCountry()(string ,error){
	//init
	var err error
	if GlobalIPInfoByIPapi == nil{
		if GlobalIPInfoByTaoBao == nil{
			err = GetInit()
			if err != nil{
				return "",err
			}
		}
	}
	//get IP for return data
	if GlobalIPInfoByIPapi != nil{
		return GlobalIPInfoByIPapi.Country,nil
	}else{
		if GlobalIPInfoByTaoBao != nil{
			return GlobalIPInfoByTaoBao.Data.Country,nil
		}else{
			return "None",nil
		}
	}
}
func GetRegionName()(string ,error){
	//init
	var err error
	if GlobalIPInfoByIPapi == nil{
		if GlobalIPInfoByTaoBao == nil{
			err = GetInit()
			if err != nil{
				return "",err
			}
		}
	}
	//get IP for return data
	if GlobalIPInfoByIPapi != nil{
		return GlobalIPInfoByIPapi.RegionName,nil
	}else{
		if GlobalIPInfoByTaoBao != nil{
			return GlobalIPInfoByTaoBao.Data.Region,nil
		}else{
			return "None",nil
		}
	}
}
//init GlobalIPInfoByIPapi
func GetIPapiInfo() (*IPAPI,error){
	var IPInfoByIPapi *IPAPI
	var err error
	for i:=0;i<3;i++{
		IPInfoByIPapi,err = IPapi()
		if IPInfoByIPapi != nil&&err ==nil{
			goto Loop
		}
	}
	Loop:
    GlobalIPInfoByIPapi = IPInfoByIPapi
    return IPInfoByIPapi,err
}

func GetIPandcountyCodeByIPapi()(string,string,error){
    return GlobalIPInfoByIPapi.IP,GlobalIPInfoByIPapi.CountryCode,nil
}
func GetIPByIPapi()(string,error){
    if GlobalIPInfoByIPapi == nil{
    	return "127.0.0.1",errors.New("GlobalIPInfoByIPapi is not exsit")
	}
    return GlobalIPInfoByIPapi.IP,nil
}
func GetcountyCodByIPapi()(string,error){
	if GlobalIPInfoByIPapi == nil{
		return "127.0.0.1",errors.New("GlobalIPInfoByIPapi is not exsit")
	}
    return GlobalIPInfoByIPapi.CountryCode,nil
}
//IPInfoByIPapi = IPapi()
//func GetBasicInfoByIPapi(typeNumber int,IPInfoByIPapi *IPAPI) string{
func GetBasicInfoByIPapi(typeNumber int) string{
    if GlobalIPInfoByIPapi == nil{
        return ""
    }
//get proposal data by number
    switch true {
    case typeNumber == 0:
        return GlobalIPInfoByIPapi.As
    case typeNumber == 1:
        return GlobalIPInfoByIPapi.City
    case typeNumber == 2:
        return GlobalIPInfoByIPapi.Country
    case typeNumber == 3:
        return GlobalIPInfoByIPapi.CountryCode
    case typeNumber == 4:
        return GlobalIPInfoByIPapi.Isp
    case typeNumber == 5:
        return strconv.FormatFloat(GlobalIPInfoByIPapi.Latitude,'f',-1,64)
    case typeNumber == 6:
        return strconv.FormatFloat(GlobalIPInfoByIPapi.Longtitude,'f',-1,64)
    case typeNumber == 7:
        return GlobalIPInfoByIPapi.Org
    case typeNumber == 8:
        return GlobalIPInfoByIPapi.IP
    case typeNumber == 9:
        return GlobalIPInfoByIPapi.Region
    case typeNumber == 10:
        return GlobalIPInfoByIPapi.RegionName
    case typeNumber == 11:
        return GlobalIPInfoByIPapi.Status
    case typeNumber == 12:
        return GlobalIPInfoByIPapi.TimeZone
    case typeNumber == 13:
        return GlobalIPInfoByIPapi.Zip
    default:
        return GlobalIPInfoByIPapi.IP
    }
    //return GlobalIPInfoByIPapi.IP
}
func get_external() string {
    resp, err := http.Get(ExternalIpURL)
    if err != nil {
        return ""
    }
    defer resp.Body.Close()
    content, _ := ioutil.ReadAll(resp.Body)
    buf := new(bytes.Buffer)
    buf.ReadFrom(resp.Body) //s := buf.String() 
    return string(content)
}
func GetIntranetIp() {
    addrs, err := net.InterfaceAddrs()
    if err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
    for _, address := range addrs {
        // 检查ip地址判断是否为回环地址
        if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
            if ipnet.IP.To4() != nil {
                fmt.Println("ip:", ipnet.IP.String())
            }
        }
    }
}
//init GlobalIPInfoByTaoBao
func GetTaobaoIpInfo()( *TaobaoIPInfo,error){
    var external_ip string
   var result *TaobaoIPInfo
   var err error
	//external_ip := get_external()
  for i:=0;i<3;i++{
      external_ip = get_external()
      if external_ip != ""{
          goto Loop
      }
  }
Loop:
	//GlobalIPAddressByTaoBao = external_ip
  	external_ip = strings.Replace(external_ip, "\n", "", -1)
  	ip := net.ParseIP(external_ip)
  	if ip == nil {
  	    fmt.Println("不是有效的IP地址！")
  	} else {
		for j:=0;j<3;j++{
			result,err = GetTaobaoInfo(string(external_ip))
			if result != nil&&err == nil {
				goto Next
			}
		}
   	}
Next:
	GlobalIPInfoByTaoBao = result
	return result,err

}
func GetTaobaoInfo(ip string) (*TaobaoIPInfo,error ){
    url := TaobooURL
    url += ip
    resp, err := http.Get(url)
    if err != nil {
        return nil,err
    }
    defer resp.Body.Close()
    out, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return nil,err
    }
    var result TaobaoIPInfo
    if err := json.Unmarshal(out, &result); err != nil {
        return nil,err
    }
    return &result,err
}
func inet_ntoa(ipnr int64) net.IP {
    var bytes [4]byte
    bytes[0] = byte(ipnr & 0xFF)
    bytes[1] = byte((ipnr >> 8) & 0xFF)
    bytes[2] = byte((ipnr >> 16) & 0xFF)
    bytes[3] = byte((ipnr >> 24) & 0xFF)
    return net.IPv4(bytes[3], bytes[2], bytes[1], bytes[0])
}
func inet_aton(ipnr net.IP) int64 {
    bits := strings.Split(ipnr.String(), ".")
    b0, _ := strconv.Atoi(bits[0])
    b1, _ := strconv.Atoi(bits[1])
    b2, _ := strconv.Atoi(bits[2])
    b3, _ := strconv.Atoi(bits[3])
    var sum int64
    sum += int64(b0) << 24
    sum += int64(b1) << 16
    sum += int64(b2) << 8
    sum += int64(b3)
    return sum
}
func IpBetween(from net.IP, to net.IP, test net.IP) bool {
    if from == nil || to == nil || test == nil {
        fmt.Println("An ip input is nil")
        return false
    }
    from16 := from.To16()
    to16 := to.To16()
    test16 := test.To16()
    if from16 == nil || to16 == nil || test16 == nil {
        fmt.Println("An ip did not convert to a 16 byte")
        return false
    }
    if bytes.Compare(test16, from16) >= 0 && bytes.Compare(test16, to16) <= 0 {
        return true
    }
    return false
}
func IsPublicIP(IP net.IP) bool {
    if IP.IsLoopback() || IP.IsLinkLocalMulticast() || IP.IsLinkLocalUnicast() {
        return false
    }
    if ip4 := IP.To4(); ip4 != nil {
        switch true {
            case ip4[0] == 10:
                return false
            case ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31:
                return false
            case ip4[0] == 192 && ip4[1] == 168:
                return false
            default:
                return true
        }
    }
    return false
}
func GetLocalIP() string {
    conn, _ := net.Dial("udp", "8.8.8.8:80")
    defer conn.Close()
    localAddr := conn.LocalAddr().String()
    idx := strings.LastIndex(localAddr, ":")
    return localAddr[0:idx]
}
