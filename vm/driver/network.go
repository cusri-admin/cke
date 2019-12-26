package driver

import (
	"cke/log"
	"cke/vm/config"
	netplug "cke/vm/driver/network"
	"cke/vm/driver/network/container"
	"cke/vm/model"
	"errors"
)

var netDrv netplug.Network

// 支持创建direct或者bridge拓扑，二者又支持共享网络(contiv)或者本地网络模式(default)
// Create Network, bridge, endpoint, etc.
func CreateNetwork(context *netplug.NetworkContext, netConf *model.NetInfo, globalConf *config.Config, vmId string) error {
	//初始化driver
	switch netConf.Type {
	case netplug.CONTIV:
		netDrv = context.Drivers[netplug.CONTIV]
		break
	case netplug.OVS:
		netDrv = context.Drivers[netplug.OVS]
		break
	default:
		netDrv = context.Drivers[netplug.DEFAULT]
	}

	// 针对VM侧尽可能按照vm的视图处理，在contiv中处理容器网络
	// 1. 基于网络参数生成网卡配置 -- bridge or direct
	if netConf.Mode == "bridge" {
		endpoint, err := createBridgeNetwork(netConf, vmId) // propagate linux bridge device
		if err != nil {
			return err
		}
		context.Endpoints[vmId] = endpoint
	} else if netConf.Mode == "direct" {
		endpoint, err := createMacvTapNetwork(netConf, vmId)
		if err != nil {
			return err
		}
		context.Endpoints[vmId] = endpoint
	} else {
		endpoint, err := createDefaultNetwork(netConf, vmId)
		if err != nil {
			return err
		}
		context.Endpoints[vmId] = endpoint
	}
	// 2. 基于配置生成相应的网络配置，返回
	return nil
}

func createBridgeNetwork(netConf *model.NetInfo, vmId string) (*netplug.Endpoint, error) {
	// libvirt create tap device
	// 创建指定模式的网络 1.创建Network 2.创建Bridge 3.创建Endpoint 4. 绑定inf到bridge

	//1
	netRes, err := netDrv.CreateNetwork(netConf)
	if err != nil || netRes.ID == "" {
		return nil, err
	}

	//2
	randomId := netplug.GenerateRandomId()
	brName := "br-vm-" + randomId
	if brName == "" {
		return nil, errors.New("Generate random id error")
	}

	err = netDrv.CreateBridge(brName)
	if err != nil {
		log.Errorf("VM [%s] Bridge [%s] error: %s", vmId, brName, err)
		return nil, err
	}

	// 基于是本地还是contiv
	endpointId := "ep-vm-" + randomId

	if netDrv.GetName() == netplug.CONTIV {
		// 3. 创建endpoint
		netRs, err := netDrv.CreateNetwork(netConf)
		if err != nil {
			// rollback
		}

		epRes, err := netDrv.CreateEndpoint(netRs, endpointId)
		if err != nil {
			// rollback
		}

		devRes, err := netDrv.BindInterfaceToBridge(brName, netRes.ID, endpointId)
		if err != nil {
			// rollback
		}
		// 4. 生成ip mac等预定信息
		epRes.EpName = devRes["name"].(string)
		epRes.Gateway = devRes["gw"].(string)
		epRes.BrName = brName
		return epRes, nil
	} else {
		//本地网络模型下 : 目前采用 dhcp 网络
		macAddr, err := netplug.GenRandomMac()
		if err != nil {
			//rollback
		}
		epRes := &netplug.Endpoint{
			Id:      endpointId,
			EpName:  endpointId,
			BrName:  brName,
			MacAddr: macAddr,
		}
		return epRes, nil
	}
	return nil, nil
}

func createMacvTapNetwork(netConf *model.NetInfo, vmId string) (*netplug.Endpoint, error) {
	// TODO: 使用macvtap直接连接veth
	// 创建指定模式的网络 1.创建Network  2.创建Endpoint 3. 绑定inf到veth
	//1
	netRes, err := netDrv.CreateNetwork(netConf)
	if err != nil || netRes.ID == "" {
		return nil, err
	}

	//2
	randomId := netplug.GenerateRandomId()

	// 基于是本地还是contiv
	endpointId := "ep-vm-" + randomId

	if netDrv.GetName() == netplug.CONTIV {
		d := netDrv.(*container.NetDriver)
		// 3. 创建endpoint
		netRs, err := d.CreateNetwork(netConf)
		if err != nil {
			// rollback
		}

		epRes, err := d.CreateEndpoint(netRs, endpointId)
		if err != nil {
			// rollback
		}

		// 4. 生成ip mac等预定信息
		joinRes, err := d.JoinEndpoint(netRes.ID, endpointId)
		if err != nil {
			// TODO: rollback
		}

		epRes.EpName = joinRes["if_name"].(string)
		epRes.Gateway = joinRes["gateway"].(string)
		return epRes, nil
	}
	return nil, nil
}

func createDefaultNetwork(netConf *model.NetInfo, vmId string) (*netplug.Endpoint, error) {
	// TODO: 本地网络使用virbr0作为依赖
	mac, _ := netplug.GenRandomMac()

	ep := &netplug.Endpoint{
		EpName:  vmId,
		MacAddr: mac,
		Id:      vmId,
		BrName:  "virbr0",
	}
	return ep, nil
}

// Delete endpoint and bridge
func DeleteNetwork(context *netplug.NetworkContext, netConf *model.NetInfo, globalConf *config.Config, vmId string) error {
	//初始化driver
	switch netConf.Type {
	case netplug.CONTIV:
		netDrv = context.Drivers[netplug.CONTIV]
	default:
		netDrv = context.Drivers[netplug.DEFAULT]
	}

	if netConf.Mode == "bridge" {
		err := deleteBridgeNetwork(netConf, vmId, context) // propagate linux bridge device
		if err != nil {
			return err
		}
		delete(context.Endpoints, vmId)
	} else if netConf.Mode == "direct" { // macvtap
		err := deleteMacvTapNetwork(netConf, vmId, context) // propagate linux bridge device
		if err != nil {
			return err
		}
		delete(context.Endpoints, vmId)
	}
	return nil
}

func deleteBridgeNetwork(netConf *model.NetInfo, vmId string, context *netplug.NetworkContext) error {
	// libvirt delete tap device
	// 创建指定模式的网络 1.从endpoint解绑定 2.删除endpoint 3.删除bridge 4.删除

	//1
	netRes, err := netDrv.CreateNetwork(netConf)
	if err != nil || netRes.ID == "" {
		return err
	}

	ep := context.Endpoints[vmId]
	brName := ep.BrName
	epName := ep.EpName
	epId := ep.Id
	err = netDrv.UnBindInterfaceFromBridge(brName, netRes.ID, epId, epName)
	if err != nil {
		log.Errorf("unbind interface error, %s", err)
		return err
	}
	err = netDrv.DeleteEndpoint(netRes, ep)
	err = netDrv.DeleteBridge(brName)
	return err
}

func deleteMacvTapNetwork(netConf *model.NetInfo, vmId string, context *netplug.NetworkContext) error {
	//1
	netRes, err := netDrv.CreateNetwork(netConf)
	if err != nil || netRes.ID == "" {
		return err
	}
	dd := netDrv.(*container.NetDriver)
	ep := context.Endpoints[vmId]
	log.Infof("Going to delete contiv endpoint: %s, %s", vmId, ep)
	dd.LeaveEndpoint(netRes.ID, ep.EpName)
	err = dd.DeleteEndpoint(netRes, ep)

	return err
}

func deleteDefaultNetwork(netConf *model.NetInfo, vmId string) error {
	return nil
}
