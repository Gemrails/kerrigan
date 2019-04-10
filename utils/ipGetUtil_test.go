package utils

import (
	"testing"
	"fmt"
	"strings"
	"net"
)
//Usage: cd pnt/utils && go test -v -run=TestRunIpGet
//Purperse: for test ipGetUtil.go function
func TestRunIpGet(t *testing.T) {
	res,err:= IPapi()
	if res != nil && err == nil{
		fmt.Println("国家：", res.Country)
		fmt.Println("国家代码：", res.CountryCode)
		fmt.Println("城市：", res.City)
		fmt.Println("运营商：", res.Isp)
		fmt.Println("时区：", res.TimeZone)
		fmt.Println("公网IP：", res.IP)
		fmt.Printf("Coordinate %v,%v\n",res.Latitude,res.Longtitude)
	}



	ip,err := GetIP()
	if err != nil{
		fmt.Printf("&&&&&&&&&&&&&&&&&&Error: %v\n",err)
	}
	fmt.Println("^^^^^^^^^^^^^^^^^^^^^IP is :",ip)

	cc,err := GetCountryCode()
	fmt.Println("^^^^^^^^^^^^^^^^^^^^^CountyCode is :",cc)
	if err != nil{
		fmt.Printf("&&&&&&&&&&&&&&&&&&Error: %v\n",err)
	}


	regionname,err := GetRegionName()
	fmt.Println("^^^^^^^^^^^^^^^^^^^^^regionname is :",regionname)
	if err != nil{
		fmt.Printf("&&&&&&&&&&&&&&&&&&Error: %v\n",err)
	}

	fmt.Println("------Line------")
	fmt.Println("test for Get IP-api info=++++++++++++++++")
	retun,err := GetIPapiInfo()
	if retun == nil || err != nil{
		fmt.Println("Get IP info by ip-api error")
	}
	for i:=0;i<14;i++{
		fmt.Println("base info : ",GetBasicInfoByIPapi(i))
	}


	fmt.Println("{{{{{{{{{{{{{{{{{{{{{{{{{+++++++++++",GlobalIPInfoByIPapi.IP)
	fmt.Println("------Line------")

	fmt.Println("++++++++++++++++++++++++++++++++++++++++++++++++++++++++")
	iip,councode,eror := GetIPandcountyCodeByIPapi()
	if iip == "" || councode==""||eror !=nil{
		fmt.Println("failed!")
	}else{
		fmt.Println("successful to get IP info by IPAPI")
	}
	fmt.Println("IP is :",iip)
	fmt.Println("countryCode is : ",councode)
	fmt.Println("++++++++++++++++++++++++++++++++++++++++++++++++++++++++")
	fmt.Println("------Line------")
	external_ip := get_external()

	fmt.Printf("external Ip  is : %v\n",external_ip)
	external_ip = strings.Replace(external_ip, "\n", "", -1)
	fmt.Println("公网ip是: ", external_ip)

	fmt.Println("------Line------")
	fmt.Println("666666666666666666666666666666666")
	result,err :=GetTaobaoIpInfo()
	if result !=nil && err ==nil{
		fmt.Println("国家：", result.Data.Country)
		//fmt.Printf("-=-=-=-=-=-=country: %v\n",result.Data.Country)
		fmt.Println("城市：", result.Data.City)
		fmt.Println("运营商：", result.Data.Isp)
		fmt.Println("公网IP：", result.Data.IP)
		fmt.Println("注册地：", result.Data.Region)
		fmt.Println("countryID：", result.Data.CountryId)
	}else{
		fmt.Println("Get Taobao Ip Inof invalid")
		fmt.Printf("Error is : %v \n",err)
	}

	//ip := net.ParseIP(external_ip)
	//if ip == nil {
	//	fmt.Println("不是有效的IP地址！")
	//} else {
	//	result,err := GetTaobaoInfo(string(external_ip))
	//	if result != nil && err == nil{
	//		fmt.Println("国家：", result.Data.Country)
	//		//fmt.Println("地区：", result.Data.Area)
	//		fmt.Println("城市：", result.Data.City)
	//		fmt.Println("运营商：", result.Data.Isp)
	//	}
	//}

	fmt.Println("------Line------")

	GetIntranetIp()

	fmt.Println("------Line------")
	//test for inet address to number and inet number to address
	ip_int := inet_aton(net.ParseIP(external_ip))
	fmt.Println("Convert IPv4 address to decimal number(base 10) :", ip_int)

	ip_result := inet_ntoa(ip_int)
	fmt.Println("Convert decimal number(base 10) to IPv4 address:", ip_result)

	fmt.Println("------Line------")

	is_between := IpBetween(net.ParseIP("0.0.0.0"), net.ParseIP("255.255.255.255"), net.ParseIP(external_ip))
	fmt.Println("check result: ", is_between)
	fmt.Println("------Line------")
	is_public_ip := IsPublicIP(net.ParseIP(external_ip))
	fmt.Println("It is public ip: ", is_public_ip)
	is_public_ip = IsPublicIP(net.ParseIP("192.168.1.1"))
	fmt.Println("It is public ip: ", is_public_ip)
	fmt.Println("------Line------")
	fmt.Println("Local Ip is: ",GetLocalIP())
}