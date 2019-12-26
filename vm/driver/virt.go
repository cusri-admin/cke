package driver

import (
	"cke/log"
	"cke/vm/driver/network"
	netC "cke/vm/driver/network/container"
	netL "cke/vm/driver/network/local"
	"cke/vm/driver/storage"
	storageL "cke/vm/driver/storage/local"
	storageR "cke/vm/driver/storage/remote"
	"cke/vm/model"
	"fmt"
	"github.com/libvirt/libvirt-go"
	"time"
)

type LibVirtDriver struct {
	conn *libvirt.Connect
}

type LibVirtContext struct {
	driver  *LibVirtDriver
	Domain  map[string]*VMDomain    //One Context contain multi-domains?
	Network *network.NetworkContext //管理网络上下文 TODO: 并存储
	Storage *storage.StorageContext //管理存储上下文 TODO: 持久化？
	State   chan VmState            //Vm state
}

// Domain和VM相关的数据结构，用于将二者关联
type VMDomain struct {
	Vm     *model.LibVm
	Domain *libvirt.Domain
}

type VmState struct {
	DomainId string
	Event    *libvirt.DomainEventLifecycle
}

var callbackId int

/**
 * Libvirt Driver. Using libvirt API manage Vms.
 */
func NewLibVirtDriver() (driver *LibVirtDriver, err error) {
	connection, err := qemuConnect()
	if err != nil {
		log.Debugf("Connecting to QEMU error: %s", err)
		return nil, err
	}
	return &LibVirtDriver{
		conn: connection,
	}, nil
}

//Context初始化：加载Etcd数据构造实际内存数据
func (driver *LibVirtDriver) InitLibVirtContext() *LibVirtContext {
	//做网络驱动的初始化
	netContext := &network.NetworkContext{
		Endpoints: make(map[string]*network.Endpoint),
		Drivers:   make(map[string]network.Network),
	}
	netC.Init(netContext, nil) //TODO： 添加全局配置参数
	netL.Init(netContext, nil)

	//做存储驱动的初始化
	storageContext := &storage.StorageContext{
		L: &storageL.DiskDriver{
			Name: "local",
		},
		R: &storageR.DiskDriver{
			Name: "remote",
			Conn: driver.conn,
		},
	}
	return &LibVirtContext{
		driver:  driver,
		Domain:  make(map[string]*VMDomain),
		Network: netContext,
		Storage: storageContext,
		State:   make(chan VmState, 1),
	}
}

func (driver *LibVirtDriver) Close() (res int, err error) {
	return driver.conn.Close()
}

func (driver *LibVirtDriver) IsAlive() (res bool, err error) {
	return driver.conn.IsAlive()
}

//qemu+ssh://root@example.com/system
func qemuConnect() (connect *libvirt.Connect, err error) {
	err = libvirt.EventRegisterDefaultImpl()
	if err != nil {
		return nil, err
	}
	return libvirt.NewConnect("qemu:///system")
}

/**
 * 列出所有的Domain信息？  TODO: 是否是Etcd的？（CKE管辖）
 */
func (driver *LibVirtDriver) ListDomains() (resStr []string, err error) {
	doms, err := driver.conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE)
	if err != nil {
		log.Errorf("List Domain error: %s", err)
		return nil, err
	}

	log.Debugf("%d running domains\n", len(doms))
	res := make([]string, len(doms))
	for i, dom := range doms {
		name, err := dom.GetName()
		if err == nil {
			res[i] = name
			log.Debugf("Domain Name: %s\n", name)
		}
		dom.Free()
	}
	return res, nil
}

/**
创建一个vm
根据业务侧需求，创建虚拟机；基本步骤如下：
（1）准备系统Boot磁盘，数据盘
（2）准备初始化配置（包括SSH，主机名等）
（3）请求网络
（4）创建VM，并返回系统状态和Console信息
*/
func (context *LibVirtContext) CreateDomain(vm *model.LibVm, taskId string) error {
	xmlConfig, err := domainXml(vm, context)
	log.Debugf("Creating domain xml : %s ", xmlConfig)
	if err != nil {
		log.Errorf("Defining domain XML: %s", err)
		return fmt.Errorf("Defining domain XML: %s", err)
	}

	callbackId = -1

	dom, err := context.driver.conn.DomainCreateXML(xmlConfig, libvirt.DOMAIN_START_PAUSED)
	if err != nil {
		return err
	}
	//TODO: 建立timeOut做回调该方法，以防止一直阻塞
	callback := func(c *libvirt.Connect, d *libvirt.Domain, event *libvirt.DomainEventLifecycle) {
		id, _ := d.GetUUIDString()
		context.State <- VmState{
			id,
			event,
		}
	}

	go func() {
		callbackId, err = context.driver.conn.DomainEventLifecycleRegister(dom, callback)
		for {
			libvirt.EventRunDefaultImpl()
		}
	}()

	time.Sleep(time.Second)

	vmDom := &VMDomain{
		Vm:     vm,
		Domain: dom,
	}
	context.Domain[taskId] = vmDom

	dom.Resume()
	if err != nil {
		log.Errorf("Creating domain with: %s\n The xml is: %s", err, xmlConfig)
		return fmt.Errorf("ERROR: Creating domain with: %s\n The xml is: %s", err, xmlConfig)
	}

	return err
}

func (context *LibVirtContext) Close() {
	context.Domain = nil
	defer func() {
		if callbackId >= 0 {
			if err := context.driver.conn.DomainEventDeregister(callbackId); err != nil {
				fmt.Errorf("got `%v` on DomainEventDeregister instead of nil", err)
			}
		}
	}()
}

func (driver *LibVirtDriver) lookupDomainByName(name string) (*libvirt.Domain, error) {
	dom, err := driver.conn.LookupDomainByName(name)
	if err != nil {
		return nil, fmt.Errorf("Cannot find the domain named: %s", name)
	}
	return dom, err
}

func (context *LibVirtContext) StartDomain(taskId string) error {
	//Fetch Domain
	dom := context.Domain[taskId].Domain
	err := dom.Resume()
	if err != nil {
		log.Errorf("Starting(RESUME) domain: %s", err)
	}
	return fmt.Errorf("ERROR: Starting(RESUME) domain: %s", err)
}

func (context *LibVirtContext) ShutDownDomain(taskId string) error {
	//Fetch Domain
	dom := context.Domain[taskId].Domain
	err := dom.Shutdown()
	if err != nil {
		log.Errorf("Shutdown(SHUTDOWN) domain: %s", err)
	}
	return fmt.Errorf("ERROR: Shutdown(SHUTDOWN) domain: %s", err)
}

func (context *LibVirtContext) PauseDomain(taskId string) error {
	//Fetch Domain
	dom := context.Domain[taskId].Domain
	err := dom.Suspend()
	if err != nil {
		log.Errorf("Pause(SUSPEND) domain: %s", err)
	}
	return fmt.Errorf("ERROR: Pause(SUSPEND) domain: %s", err)
}

func (context *LibVirtContext) KillDomain(taskId string) error {
	//Fetch Domain
	dom := context.Domain[taskId].Domain
	vm := context.Domain[taskId].Vm

	err := dom.Destroy()
	if err != nil {
		log.Errorf("Kill(DESTROY) domain: %s", err)
	}
	//TODO: 回退网络
	DeleteNetwork(context.Network, vm.Network, nil, vm.Id)
	//TODO: 回退存储
	DeleteDisk(vm, context.Storage)

	return fmt.Errorf("ERROR: Kill(DESTROY) domain: %s", err)
}
