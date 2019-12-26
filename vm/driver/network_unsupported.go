package driver

import (
	"cke/log"
	"cke/vm/config"
	netplug "cke/vm/driver/network"
	"cke/vm/model"
	"github.com/pkg/errors"
)

//Create Network, bridge, endpoint, etc.
func fakeCreateNetwork(context *netplug.NetworkContext, netConf *model.NetInfo, globalConf *config.Config, vmId string) error {
	var driver netplug.Network
	//初始化driver
	switch netConf.Type {
	case netplug.CONTIV:
		driver = context.Drivers[netplug.CONTIV]
	default:
		driver = context.Drivers[netplug.DEFAULT]
	}

	// 创建指定模式的网络 1.创建Network 2.创建Bridge 3.创建Endpoint 4. 绑定inf到bridge

	//1
	netRes, err := driver.CreateNetwork(netConf)
	if err != nil || netRes.ID == "" {
		return err
	}

	//2
	randomId := netplug.GenerateRandomId()
	brName := "br-vm-" + randomId
	if brName == "" {
		err = errors.New("Generate random id error")
		return err
	}

	//nsName := "ns-vm-" + randomId
	//nsPath := netplug.GenerateNameSpacePath(nsName)
	err = driver.CreateBridge(brName)
	if err != nil {
		log.Errorf("VM [%s] Bridge [%s] error: %s", vmId, brName, err)
		return err
	}

	//3
	//epName := "ep-vm-" + randomId
	//inf, err := driver.CreateEndpoint(netRes, epName)
	if err != nil {
		//删除bridge
		driver.DeleteBridge(brName)
		return err
	}
	//infIpv4Addr := inf.(*netapi.EndpointInterface).Address
	// infIpv6Addr := inf.(*netapi.EndpointInterface).AddressIPv6
	// infMacAddr := inf.(*netapi.EndpointInterface).MacAddress

	//4

	//last return
	endpoint := &netplug.Endpoint{
		Id:     vmId,
		BrName: brName,
		// Ipv4Addr: infIpv4Addr,
		// Ipv6Addr: infIpv6Addr,
		// MacAddr:  infMacAddr,
	}
	context.Endpoints[vmId] = endpoint
	return err
}

func fakeCreateBridge() {

}

// Delete endpoint and bridge
func fakeDeleteNetwork() {

}
