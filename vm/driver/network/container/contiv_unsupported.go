package container

import (
	"cke/log"
	netplug "cke/vm/driver/network"
	"cke/vm/model"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/libnetwork/driverapi"
	netapi "github.com/docker/libnetwork/drivers/remote/api"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"runtime"
	"strings"
)

func (d *NetDriver) createEndpoint(networkInfo *types.NetworkResource, endpointID string) (interface{}, error) {
	//申请获取endpoint信息 CreateEndpoint
	method := driverapi.NetworkPluginEndpointType + ".CreateEndpoint"
	infoCreate := &netapi.CreateEndpointRequest{
		NetworkID:  networkInfo.ID,
		EndpointID: endpointID,
		Interface:  &netapi.EndpointInterface{},
		Options:    nil,
	}
	var resCreate netapi.CreateEndpointResponse
	d.NetClient.CallWithOptions(method, infoCreate, &resCreate)
	if resCreate.Err != "" {
		log.Errorf("Creating endpoint error: %s", resCreate.Err)
		return nil, errors.New(resCreate.Err)
	}
	return resCreate.Interface, nil
}

func (d *NetDriver) deleteEndpoint(networkInfo *types.NetworkResource, endpointID string) error {
	//申请获取endpoint信息 DeleteEndpoint
	method := driverapi.NetworkPluginEndpointType + ".DeleteEndpoint"
	infoDelete := &netapi.DeleteEndpointRequest{
		NetworkID:  networkInfo.ID,
		EndpointID: endpointID,
	}

	var resDelete netapi.DeleteEndpointResponse
	d.NetClient.CallWithOptions(method, infoDelete, &resDelete)
	if resDelete.Err != "" {
		log.Errorf("Delete endpoint error: %s", resDelete.Err)
		return errors.New(resDelete.Err)
	}
	return nil
}

// CreateNetwork 创建Network
func (d *NetDriver) createNetwork(netConf *model.NetInfo) (*types.NetworkResource, error) {
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

func (d *NetDriver) deleteNetwork() error { return nil }

//创建删除Tap设备
func (d *NetDriver) createVMTap() error { return nil }
func (d *NetDriver) deleteVMTap() error { return nil }

// CreateBridge create linux bridge in given namespace
func (d *NetDriver) createBridge(brName string, nsPath string) error {
	// Lock the OS Thread so we don't accidentally switch namespaces
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Save the current network namespace
	origns, _ := netns.Get()
	defer origns.Close()

	//create namespace
	err := netplug.CreateNetworkNamespace(nsPath, true)
	if err != nil {
		return err
	}

	newns, err := netns.GetFromPath(nsPath)
	if err != nil {
		return err
	}

	if err = netns.Set(newns); err != nil {
		return err
	}
	defer newns.Close()

	// --------in new namespace-------
	la := netlink.NewLinkAttrs()
	la.Name = brName
	bridge := &netlink.Bridge{LinkAttrs: la}

	if err := netlink.LinkAdd(bridge); err != nil {
		return err
	}
	if err := netlink.LinkSetUp(bridge); err != nil {
		return err
	}

	list, err := netlink.LinkList()
	for _, link := range list {
		fmt.Println(link.Attrs(), link.Type())
	}

	// -------out new namespace--------
	defer netns.Set(origns)
	return nil
}

func (d *NetDriver) deleteBridge(brName string, nsPath string) error {
	// Lock the OS Thread so we don't accidentally switch namespaces
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Save the current network namespace
	origns, _ := netns.Get()
	defer origns.Close()

	//checkout namespace
	newns, err := netns.GetFromPath(nsPath)
	if err != nil {
		return err
	}
	netns.Set(newns)
	defer newns.Close()
	// --------in new namespace-------
	la := netlink.NewLinkAttrs()
	la.Name = brName
	bridge := &netlink.Bridge{LinkAttrs: la}
	if err := netlink.LinkSetDown(bridge); err != nil {
		netns.Set(origns)
		return err
	}

	err = netlink.LinkDel(bridge)
	if err != nil {
		netns.Set(origns)
		return err
	}
	list, _ := netlink.LinkList()
	for _, link := range list {
		fmt.Println(link.Attrs(), link.Type())
	}
	// -------out new namespace--------
	netns.Set(origns)
	netplug.RemoveNetworkNamespace(nsPath)
	return nil
}

//BindInterfaceToBridge 绑定与解绑 桥与interface
func (d *NetDriver) bindInterfaceToBridge(brName string, netId string, endpointID string, nsPath string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Save the current network namespace
	origns, _ := netns.Get()
	defer origns.Close()

	// docker join 将 veth 加入 ns
	method := driverapi.NetworkPluginEndpointType + ".Join"
	joinReq := &netapi.JoinRequest{
		NetworkID:  netId,
		EndpointID: endpointID,
		SandboxKey: nsPath,
	}

	var resJoin netapi.JoinResponse
	d.NetClient.CallWithOptions(method, joinReq, &resJoin)
	if resJoin.Err != "" {
		log.Errorf("Join endpoint error: %s", resJoin.Err)
		return errors.New(resJoin.Err)
	}

	//checkout namespace
	newns, err := netns.GetFromPath(nsPath)
	if err != nil {
		return err
	}

	// 执行join sandbox 操作。。。
	ifaceName := resJoin.InterfaceName
	fmt.Println(ifaceName.DstName + "---" + ifaceName.SrcName + "---" + ifaceName.DstPrefix)
	vportInf, err := netlink.LinkByName(ifaceName.SrcName)
	nlh := &netlink.Handle{}
	err = nlh.LinkSetNsFd(vportInf, int(newns))
	if err != nil {
		err = nlh.LinkSetNsFd(vportInf, int(origns))
		log.Errorf("moving interface %s to host ns failed, after config error %v", vportInf, err)
		return err
	}

	netns.Set(newns)
	defer newns.Close()
	// --------in new namespace-------
	bridge, err := netlink.LinkByName(brName)
	vBridge := bridge.(*netlink.Bridge)

	if err != nil {
		return err
	}
	// 绑定ns下的veth到bridge
	// 找到veth
	var dev netlink.Link
	links, err := netlink.LinkList()
	for _, link := range links {
		fmt.Println(link)
		if strings.HasPrefix(link.Attrs().Name, "vport") {
			dev = link
			break
		}
	}

	if dev == nil {
		log.Error("Could not find contiv vport device")
		return errors.New("Could not find contiv vport device")
	}

	err = netlink.LinkSetMaster(dev, vBridge)
	if err != nil {
		return errors.New("Could not add dev " + dev.Attrs().Name + " to bridge " + vBridge.Name)
	}
	err = netlink.LinkSetUp(dev)
	if err != nil {
		return errors.New("Could not set " + dev.Attrs().Name + " up")
	}

	// -------out new namespace--------
	defer netns.Set(origns)
	return nil
}

func (d *NetDriver) unBindInterfaceFromBridge(brName string, netId string, endpointID string, nsPath string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Save the current network namespace
	origns, _ := netns.Get()
	defer origns.Close()

	//checkout namespace
	newns, err := netns.GetFromPath(nsPath)
	if err != nil {
		return err
	}
	netns.Set(newns)
	defer newns.Close()
	// --------in new namespace-------

	// 解绑ns下的veth
	// 找到veth
	var dev netlink.Link
	links, err := netlink.LinkList()
	for _, link := range links {
		fmt.Println(link)
		if strings.HasPrefix(link.Attrs().Name, "vport") {
			dev = link
			break
		}
	}

	if dev == nil {
		log.Error("Could not find contiv vport device")
		return errors.New("Could not find contiv vport device")
	}
	netlink.LinkSetDown(dev)
	err = netlink.LinkSetNoMaster(dev)
	if err != nil {
		return err
	}
	// -------out new namespace--------
	nlh := &netlink.Handle{}
	nlh.LinkSetNsFd(dev, int(origns))
	if err != nil {
		log.Errorf("moving interface %s to host ns failed, after config error %v", dev, err)
		return err
	}
	netns.Set(origns)
	defer func() {
		// docker leave 将 veth 移出 ns

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
	}()

	return nil
}
