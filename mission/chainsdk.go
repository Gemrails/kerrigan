package main

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/core"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
)

const (
	channelID      = "default"
	orgName        = "bcs06071508"
	orgAdmin       = "Admin"
	ordererOrgName = "bcs06071508"
	ccID           = "obcs-example02"
)

// ExampleCC query and transaction arguments
var queryArgs = [][]byte{[]byte("query"), []byte("b")}
var txArgs = [][]byte{[]byte("move"), []byte("a"), []byte("b"), []byte("1")}

func setupAndRun(configOpt core.ConfigProvider, sdkOpts ...fabsdk.Option) {
	//Init the sdk config
	sdk, err := fabsdk.New(configOpt, sdkOpts...)
	if err != nil {
		fmt.Println("Failed to create new SDK: %s", err)
	}
	defer sdk.Close()
	// ************ setup complete ************** //

	//prepare channel client context using client context
	clientChannelContext := sdk.ChannelContext(channelID, fabsdk.WithUser("Admin"), fabsdk.WithOrg(orgName))

	// Channel client is used to query and execute transactions (Org1 is default org)
	client, err := channel.New(clientChannelContext)

	if err != nil {
		fmt.Println("Failed to create new channel client: %s", err)
	}

	value := queryCC(client)
	fmt.Println("value is d%", int64(binary.BigEndian.Uint64(value)))

	eventID := "test([a-zA-Z]+)"

	// Register chaincode event (pass in channel which receives event details when the event is complete)
	reg, notifier, err := client.RegisterChaincodeEvent(ccID, eventID)
	if err != nil {
		fmt.Println("Failed to register cc event: %s", err)
	}
	defer client.UnregisterChaincodeEvent(reg)

	// Move funds
	executeCC(client)

	select {
	case ccEvent := <-notifier:
		fmt.Println("Received CC event: %#v\n", ccEvent)
	case <-time.After(time.Second * 20):
		fmt.Println("Did NOT receive CC event for eventId(%s)\n", eventID)
	}
}

func executeCC(client *channel.Client) {
	_, err := client.Execute(channel.Request{ChaincodeID: ccID, Fcn: "invoke", Args: txArgs},
		channel.WithRetry(retry.DefaultChannelOpts))
	if err != nil {
		fmt.Println("Failed to move funds: %s", err)
	}
}

func queryCC(client *channel.Client) []byte {
	response, err := client.Query(channel.Request{ChaincodeID: ccID, Fcn: "invoke", Args: queryArgs},
		channel.WithRetry(retry.DefaultChannelOpts))
	if err != nil {
		fmt.Println("Failed to query funds: %s", err)
	}
	fmt.Println(response)

	return response.Payload
}

func main() {
	configPath := "./config.yml"
	//End to End testing
	setupAndRun(config.FromFile(configPath))
}
