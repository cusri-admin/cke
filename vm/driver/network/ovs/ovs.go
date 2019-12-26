package ovs

import (
	"cke/vm/config"
	netplug "cke/vm/driver/network"
	"cke/vm/model"
	"github.com/docker/docker/api/types"
)

type driver struct {
	Name string
	Type string
}

func newDriver() *driver {
	return &driver{
		Name: netplug.OVS,
		Type: netplug.OVS,
	}
}

func Init(ctx *netplug.NetworkContext, globalConf *config.Config) {

}

func (d *driver) GetName() string {
	return ""
}

//创建删除Endpoint{}
func (d *driver) CreateEndpoint(networkInfo *types.NetworkResource, endpointID string) (*netplug.Endpoint, error) {
	return nil, nil
}

func (d *driver) DeleteEndpoint(networkInfo *types.NetworkResource, endpoint *netplug.Endpoint) error {
	return nil
}

// CreateNetwork 创建删除Network{}
func (d *driver) CreateNetwork(netConf *model.NetInfo) (*types.NetworkResource, error) {
	return nil, nil
}

func (d *driver) DeleteNetwork() error {
	return nil
}

//创建删除Tap设备
func (d *driver) CreateVMTap() error {
	return nil
}

func (d *driver) DeleteVMTap() error {
	return nil
}

//创建删除bridge{}
func (d *driver) CreateBridge(brName string) error {
	return nil
}

func (d *driver) DeleteBridge(brName string) error {
	return nil
}

//绑定与解绑 桥与interface
func (d *driver) BindInterfaceToBridge(brName string, netId string, epId string) (map[string]interface{}, error) {
	return nil, nil
}

func (d *driver) UnBindInterfaceFromBridge(brName string, netId string, endpointID string, ifName string) error {
	return nil
}
