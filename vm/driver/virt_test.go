package driver

import (
	"cke/log"
	"cke/vm/model"
	"fmt"
	"testing"
	"time"
)

// Contiv Ceph 测试
func TestContivCephLibvirtDriver(t *testing.T) {
	libVirtDriver, err := NewLibVirtDriver()
	defer libVirtDriver.Close()

	context := libVirtDriver.InitLibVirtContext()
	defer context.Close()

	if err != nil {
		fmt.Println("Ops.... Error")
	}

	res, err := libVirtDriver.ListDomains()
	log.Debugf("List Domain Res : %s, ERROR: %s.", res, err)

	// 构造生成vm libvirt xml文件
	vm := &model.LibVm{
		Name:   "instance-test",
		Cpus:   1,
		Mem:    2048,
		Region: "10.124.142.225",
		OsInfo: map[string]string{
			"imgPath": "/data/CentOS-7-x86_64-GenericCloud-1809.qcow2",
		},
		Network: &model.NetInfo{
			Name: "ckenet",
			Type: "contiv",
			Mode: "bridge",
		},
		BootType: model.BootType_BOOT_FILE,
		Disk: &model.VDisk{
			SystemDisk: &model.VmDisk{
				Name: "bootVol",
				Size: 10737418240,
			},
		},
		Label: map[string]string{
			"showVnc":  "yes",
			"tenant":   "chen",
			"poolName": "welkin-ceph-chen",
		},
	}
	err = context.CreateDomain(vm, "testTaskId")
	if err != nil {
		t.Errorf("Creating domain error occured: %s", err)
	}

	select {
	case s := <-context.State:
		fmt.Println(s)
	case <-time.After(time.Second * 10):
		fmt.Println("time out with on events....return")
		return
	}
	defer func() {
		context.KillDomain("testTaskId")
	}()
}

// 本地网络本地磁盘测试
func TestLocalNetLocalStorageLibvirtDriver(t *testing.T) {
	libVirtDriver, err := NewLibVirtDriver()
	defer libVirtDriver.Close()

	context := libVirtDriver.InitLibVirtContext()
	defer context.Close()

	if err != nil {
		t.Errorf("Libvirt Driver Init ERROR: %s", err)
	}

	res, err := libVirtDriver.ListDomains()
	t.Logf("List Domain Res : %s, ERROR: %s.", res, err)

	// 构造生成vm libvirt xml文件
	vm := &model.LibVm{
		Name:   "instance-test",
		Cpus:   1,
		Mem:    2048,
		Region: "10.124.142.225",
		OsInfo: map[string]string{
			"imgPath": "/data/CentOS-7-x86_64-GenericCloud-1809.qcow2",
		},
		Network: &model.NetInfo{
			Name: "test",
			Type: "default",
			Mode: "direct",
		},
		BootType: model.BootType_BOOT_FILE,
		Disk: &model.VDisk{
			SystemDisk: &model.VmDisk{
				Name: "bootVol",
				Size: 10737418240,
			},
		},
		Label: map[string]string{
			"showVnc": "yes",
			"tenant":  "chen",
		},
	}
	err = context.CreateDomain(vm, "testTaskId")
	if err != nil {
		t.Errorf("Creating domain error occured: %s", err)
	}

	select {
	case s := <-context.State:
		fmt.Println(s)
	case <-time.After(time.Second * 10):
		fmt.Println("time out with on events....return")
		return
	}

	time.Sleep(time.Second * 10)
	defer func() {
		context.KillDomain("testTaskId")
	}()

}
