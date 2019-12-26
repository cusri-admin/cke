package local

import (
	"cke/vm/config"
	netplug "cke/vm/driver/network"
	"cke/vm/model"
	"github.com/docker/docker/api/types"
	"github.com/vishvananda/netlink"
	"net"
)

/*****************
  Author: Chen
  Date: 2019/4/23
  Comp: ChinaUnicom
 ****************/
type driver struct {
	Name string
	Type string
}

func newDriver(name string) *driver {
	return &driver{
		Name: name,
		Type: "docker",
	}
}

func Init(context *netplug.NetworkContext, globalConf *config.Config) {
	driver := newDriver(netplug.DEFAULT)
	context.RegisterNetwork(netplug.DEFAULT, driver)
}

func createBridgeIface(name string, addr *net.IPNet) error {
	la := netlink.NewLinkAttrs()
	la.Name = name
	bridge := &netlink.Bridge{LinkAttrs: la}
	if err := netlink.LinkAdd(bridge); err != nil {
		return err
	}
	if err := netlink.AddrAdd(bridge, &netlink.Addr{IPNet: addr}); err != nil {
		return err
	}
	return netlink.LinkSetUp(bridge)
}

func (d *driver) GetName() string {
	return netplug.DEFAULT
}

func (*driver) CreateEndpoint(networkInfo *types.NetworkResource, endpointID string) (*netplug.Endpoint, error) {
	return nil, nil
}

func (*driver) DeleteEndpoint(networkInfo *types.NetworkResource, endpoint *netplug.Endpoint) error {
	return nil
}

//创建删除Network
func (*driver) CreateNetwork(netConf *model.NetInfo) (*types.NetworkResource, error) {
	return nil, nil
}

func (*driver) DeleteNetwork() error { return nil }

//创建删除Tap设备
func (*driver) CreateVMTap() error { return nil }
func (*driver) DeleteVMTap() error { return nil }

//创建删除bridge
func (*driver) CreateBridge(brName string) error { return nil }
func (*driver) DeleteBridge(brName string) error { return nil }

//绑定与解绑 桥与interface
func (*driver) BindInterfaceToBridge(brName string, netId string, epId string) (map[string]interface{}, error) {
	return nil, nil
}

func (*driver) UnBindInterfaceFromBridge(brName string, netId string, endpointID string, ifName string) error {
	return nil
}
