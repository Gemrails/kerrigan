package main

import (
	"fmt"
	"log"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
)

func main() {

	//读取配置文件，创建SDK
	// configProvider := config.FromFile("./config.yaml")
	sdk, err := fabsdk.New(nil)
	if err != nil {
		log.Fatalf("create sdk fail: %s\n", err.Error())
	}

	//读取配置文件(config.yaml)中的组织(member1.example.com)的用户(Admin)
	mspClient, err := msp.New(sdk.Context(), msp.WithOrg(""))
	if err != nil {
		log.Fatalf("create msp client fail: %s\n", err.Error())
	}

	adminIdentity, err := mspClient.GetSigningIdentity("Admin")
	if err != nil {
		log.Fatalf("get admin identify fail: %s\n", err.Error())
	} else {
		fmt.Println("AdminIdentify is found:")
		fmt.Println(adminIdentity)
	}

	//调用合约
	channelProvider := sdk.ChannelContext("myc",
		fabsdk.WithUser("Admin"),
		fabsdk.WithOrg(""))

	channelClient, err := channel.New(channelProvider)
	if err != nil {
		log.Fatalf("create channel client fail: %s\n", err.Error())
	}

	var args [][]byte
	args = append(args, []byte("key1"))

	request := channel.Request{
		ChaincodeID: "mycc",
		Fcn:         "query",
		Args:        args,
	}
	response, err := channelClient.Query(request)
	if err != nil {
		log.Fatal("query fail: ", err.Error())
	} else {
		fmt.Printf("response is %s\n", response.Payload)
	}
}
