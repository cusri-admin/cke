package container

import (
	"cke/log"
	"cke/vm/config"
	netplug "cke/vm/driver/network"
	"cke/vm/model"
	"context"
	"errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/libnetwork/driverapi"
	netapi "github.com/docker/libnetwork/drivers/remote/api"
	"github.com/docker/libnetwork/ipamapi"
	ipam "github.com/docker/libnetwork/ipams/remote/api"
	"github.com/vishvananda/netlink"
    "net"
    "strconv"
    "strings"
)

type NetDriver struct {
	IpamClient *plugins.Client
	NetClient  *plugins.Client
	Name       string
	Type       string
}

func newDriver(nc *plugins.Client, ipc *plugins.Client) *NetDriver {
	return &NetDriver{
		IpamClient: ipc,
		NetClient:  nc,
		Name:       netplug.CONTIV,
		Type:       "docker",
	}
}

// Init intialize the contiv driver
func Init(ctx *netplug.NetworkContext, globalConf *config.Config) {
	netPlugin, err := plugins.Get(netplug.NETPLUGIN, driverapi.NetworkPluginEndpointType)
	if err != nil {
		log.Errorf("Cannot connect to contiv network plugin [%s]", err)
		return
	}

	ipamPlugin, err := plugins.Get(netplug.NETPLUGIN, ipamapi.PluginEndpointType)
	if err != nil {
		log.Errorf("Cannot connect to contiv network plugin [%s]", err)
		return
	}

	netClient := netPlugin.Client()   // Network Client
	ipamClient := ipamPlugin.Client() // IPAM Client
	driver := newDriver(netClient, ipamClient)
	ctx.RegisterNetwork(netplug.CONTIV, driver)
}

func (d *NetDriver) GetName() string {
	return netplug.CONTIV
}

func (d *NetDriver) requestIPPool(ipams network.IPAM) (string, error) {
	method := ipamapi.PluginEndpointType + ".RequestPool"
	poolReq := &ipam.RequestPoolRequest{
		Options: ipams.Options,
		Pool:    ipams.Config[0].Subnet,
	}
	poolRes := &ipam.RequestPoolResponse{}
	d.IpamClient.CallWithOptions(method, poolReq, poolRes)
	if poolRes.IsSuccess() && poolRes.PoolID != "" {
		return poolRes.PoolID, nil
	}
	return "", errors.New(poolRes.GetError())
}

func (d *NetDriver) requestAddr(poolID string) (string, error) {
	method := ipamapi.PluginEndpointType + ".RequestAddress"
	addReq := &ipam.RequestAddressRequest{
		PoolID: poolID,
	}
	addRes := &ipam.RequestAddressResponse{}
	d.IpamClient.CallWithOptions(method, addReq, addRes)
	if addRes.IsSuccess() && addRes.Address != "" {
		return addRes.Address, nil
	}
	return "", errors.New(addRes.GetError())
}

func (d *NetDriver) releaseAddr(poolID string, address string) error {
	method := ipamapi.PluginEndpointType + ".ReleaseAddress"
	reAddrReq := &ipam.ReleaseAddressRequest{
		PoolID:  poolID,
		Address: address,
	}
	reAddrRes := &ipam.ReleaseAddressResponse{}
	d.IpamClient.CallWithOptions(method, reAddrReq, reAddrRes)
	if reAddrRes.IsSuccess() && reAddrRes.Error == "" {
		return nil
	}
	return errors.New(reAddrRes.GetError())
}

func (d *NetDriver) CreateEndpoint(networkInfo *types.NetworkResource, endpointID string) (*netplug.Endpoint, error) {
	//申请获取endpoint信息 CreateEndpoint
	method := driverapi.NetworkPluginEndpointType + ".CreateEndpoint"

	ipPoolID, err := d.requestIPPool(networkInfo.IPAM)
	if err != nil {
		return nil, err
	}

	addr, err := d.requestAddr(ipPoolID)
	if err != nil {
		return nil, err
	}

	infoCreate := &netapi.CreateEndpointRequest{
		NetworkID:  networkInfo.ID,
		EndpointID: endpointID,
		Interface: &netapi.EndpointInterface{
			Address: addr,
		},
		Options: nil,
	}
	var resCreate netapi.CreateEndpointResponse
	d.NetClient.CallWithOptions(method, infoCreate, &resCreate)
	if resCreate.Err != "" {
		log.Errorf("Creating endpoint error: %s", resCreate.Err)
		return nil, errors.New(resCreate.Err)
	}
	dockerEp := resCreate.Interface
    ipnet, mask, err:= parseCIDR(addr)
    if err != nil {
        log.Errorf("Parse CIDR IP error: %s", err)
        return nil, err
    }

	ep := &netplug.Endpoint{
		Id:       endpointID,
		Ipv4Addr: strings.Split(addr, "/")[0],
		Ipv4Net:  ipnet,
		Ipv4Mask: mask,
		Ipv6Addr: dockerEp.AddressIPv6,
		MacAddr:  dockerEp.MacAddress,
	}
	return ep, nil
}

func (d *NetDriver) DeleteEndpoint(networkInfo *types.NetworkResource, endpoint *netplug.Endpoint) error {
	//申请获取endpoint信息 DeleteEndpoint
	method := driverapi.NetworkPluginEndpointType + ".DeleteEndpoint"
	infoDelete := &netapi.DeleteEndpointRequest{
		NetworkID:  networkInfo.ID,
		EndpointID: endpoint.Id,
	}

	var resDelete netapi.DeleteEndpointResponse
	d.NetClient.CallWithOptions(method, infoDelete, &resDelete)
	if resDelete.Err != "" {
		log.Errorf("Delete endpoint error: %s", resDelete.Err)
		return errors.New(resDelete.Err)
	}

	// TODO: 移除IP地址
	// d.releaseAddr(poolID, )
	ipPoolID, err := d.requestIPPool(networkInfo.IPAM)
	if err != nil {
		return err
	}

	d.releaseAddr(ipPoolID, endpoint.Ipv4Addr)
	return nil
}

// CreateNetwork 创建Network
func (d *NetDriver) CreateNetwork(netConf *model.NetInfo) (*types.NetworkResource, error) {
	//contiv默认使用docker网络，不创建网络，该接口返回租户对应的contiv网络
	//通过网络名称获取网络id
	networkInfo, err := GetDockerNetworkInfoByName(netConf.Name)
	if err != nil {
		log.Errorf("Fetch network id by name [%s] error : %s ", netConf.Name, err)
		return nil, err
	}
	//返回ID
	return networkInfo, nil
}

func (d *NetDriver) DeleteNetwork() error { return nil }

//创建删除Tap设备
func (d *NetDriver) CreateVMTap() error { return nil }
func (d *NetDriver) DeleteVMTap() error { return nil }

// CreateBridge create linux bridge
func (d *NetDriver) CreateBridge(brName string) error {
	la := netlink.NewLinkAttrs()
	la.Name = brName
	bridge := &netlink.Bridge{LinkAttrs: la}

	if err := netlink.LinkAdd(bridge); err != nil {
		return err
	}

	if err := netlink.LinkSetUp(bridge); err != nil {
		return err
	}

	return nil
}

func (d *NetDriver) DeleteBridge(brName string) error {
	la := netlink.NewLinkAttrs()
	la.Name = brName
	bridge := &netlink.Bridge{LinkAttrs: la}

	if err := netlink.LinkSetDown(bridge); err != nil {
		return err
	}

	if err := netlink.LinkDel(bridge); err != nil {
		return err
	}

	return nil
}

func (d *NetDriver) JoinEndpoint(netId string, endpointID string) (map[string]interface{}, error) {
	// docker join 将 veth 加入
	method := driverapi.NetworkPluginEndpointType + ".Join"
	joinReq := &netapi.JoinRequest{
		NetworkID:  netId,
		EndpointID: endpointID,
	}

	var resJoin netapi.JoinResponse
	d.NetClient.CallWithOptions(method, joinReq, &resJoin)
	if resJoin.Err != "" {
		log.Errorf("Join endpoint error: %s", resJoin.Err)
		return nil, errors.New(resJoin.Err)
	}
	infRes := map[string]interface{}{
		"if_name":      resJoin.InterfaceName.SrcName,
		"gateway":      resJoin.Gateway,
		"gateway_ipv6": resJoin.GatewayIPv6,
		"route":        resJoin.StaticRoutes,
	}
	return infRes, nil
}

func (d *NetDriver) LeaveEndpoint(netId string, endpointID string) {
	// docker leave 将 veth 移出
	method := driverapi.NetworkPluginEndpointType + ".Leave"
	leaveReq := &netapi.LeaveRequest{
		NetworkID:  netId,
		EndpointID: endpointID,
	}

	var resLeave netapi.LeaveResponse
	d.NetClient.CallWithOptions(method, leaveReq, &resLeave)
	if resLeave.Err != "" {
		log.Errorf("Leave sandbox error: %s", resLeave.Err)
	}
}

//BindInterfaceToBridge 绑定与解绑 桥与interface
func (d *NetDriver) BindInterfaceToBridge(brName string, netId string, endpointID string) (map[string]interface{}, error) {
	// docker join 将 veth 加入
	resJoin, err := d.JoinEndpoint(netId, endpointID)
	if err != nil {
		return nil, err
	}

	// 执行join sandbox 操作。。。
	ifaceName := resJoin["name"].(netapi.InterfaceName)
	log.Debugf("%s --- %s --- %s", ifaceName.DstName, ifaceName.SrcName, ifaceName.DstPrefix)

	vportInf, err := netlink.LinkByName(ifaceName.SrcName)
	if err != nil {
		log.Errorf("moving interface %s to host ns failed, after config error %v", vportInf, err)
		return nil, err
	}

	bridge, err := netlink.LinkByName(brName)
	vBridge := bridge.(*netlink.Bridge)

	if err != nil {
		return nil, err
	}

	if err = netlink.LinkSetMaster(vportInf, vBridge); err != nil {
		return nil, errors.New("Could not add dev " + vportInf.Attrs().Name + " to bridge " + vBridge.Name)
	}

	if err = netlink.LinkSetUp(vportInf); err != nil {
		return nil, errors.New("Could not set " + vportInf.Attrs().Name + " up")
	}

	resInfo := map[string]interface{}{
		"name": vportInf.Attrs().Name,
		"gw":   resJoin["gateway"],
	}
	return resInfo, nil
}

func (d *NetDriver) UnBindInterfaceFromBridge(brName string, netId string, endpointID string, ifName string) (err error) {
	// 找到veth
	var dev netlink.Link

	dev, err = netlink.LinkByName(ifName)
	if err != nil {
		log.Error("Could not find contiv vport device")
		return errors.New("Could not find contiv vport device")
	}

	netlink.LinkSetDown(dev)
	err = netlink.LinkSetNoMaster(dev)

	if err != nil {
		log.Errorf("moving interface %s to host ns failed, after config error %v", dev, err)
		return
	}

	defer func() {
		d.LeaveEndpoint(netId, endpointID)
	}()

	return
}

// GetDockerNetworkInfoByName 从netName中获取netId及IPAM信息
func GetDockerNetworkInfoByName(nwName string) (*types.NetworkResource, error) {
	docker, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv)
	if err != nil {
		log.Errorf("Unable to connect to docker. Error %v", err)
		return nil, errors.New("unable to connect to docker")
	}

	nwIDFilter := filters.NewArgs()
	nwIDFilter.Add("name", nwName)
	nwList, err := docker.NetworkList(context.Background(), types.NetworkListOptions{Filters: nwIDFilter})
	if err != nil {
		log.Infof("Error: %v", err)
		return nil, err
	}

	if len(nwList) != 1 {
		if len(nwList) == 0 {
			err = errors.New("network Name not found")
		} else {
			err = errors.New("more than one network found with the same name")
		}
		return nil, err
	}
	nw := nwList[0]
	return &nw, nil
}

func parseCIDR(address string) (ip string, mask string, err error) {
    _, ipNet, err:= net.ParseCIDR(address)
    if err != nil {
        return "", "", err
    }

    val := make([]byte, len(ipNet.Mask))
    copy(val, ipNet.Mask)

    var s []string
    for _, i := range val[:] {
        s = append(s, strconv.Itoa(int(i)))
    }
    return ipNet.IP.String(), strings.Join(s, "."), nil
}