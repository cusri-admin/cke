package driver

import (
	"cke/vm/model"
	"encoding/json"
	"fmt"
	"testing"
)

// 测试Contiv Ceph 虚机 xml
func TestDomainXml(t *testing.T) {
	d, _ := NewLibVirtDriver()
	ctx := d.InitLibVirtContext()

	vmTest := &model.LibVm{
		Name: "test",
		Cpus: 1,
		Mem:  2 * 1024,
		OsInfo: map[string]string{
			"imgPath": "/data/CentOS-7-x86_64-GenericCloud-1809.qcow2",
		},
		Network: &model.NetInfo{
			Name: "ckenet",
			Type: "contiv",
			Mode: "bridge",
		},
		BootType: model.BootType_BOOT_FILE,
		Label: map[string]string{
			"showVnc":  "yes",
			"tenant":   "chen",
			"poolName": "welkin-ceph-chen",
		},
		Disk: &model.VDisk{
			SystemDisk: &model.VmDisk{
				Name: "bootVol",
				Size: 10737418240,
			},
		},
		Host: "10.124.142.225",
	}
	data, _ := json.Marshal(vmTest)
	fmt.Println(string(data[:]))
	xmlString, err := domainXml(vmTest, ctx)

	if err != nil {
		fmt.Println(err)
		t.Error(err)
	}

	fmt.Println(xmlString)
	t.Log("Testing: Creating domain xml success!")
}

// 测试本地 虚机 xml
func TestLocalDomainXml(t *testing.T) {
	d, _ := NewLibVirtDriver()
	ctx := d.InitLibVirtContext()

	vmTest := &model.LibVm{
		Name: "test",
		Cpus: 1,
		Mem:  2 * 1024,
		Host: "10.124.142.224",
		OsInfo: map[string]string{
			"imgPath": "/data/CentOS-7-x86_64-GenericCloud-1809.qcow2",
		},
		Network: &model.NetInfo{
			Name: "test",
			Type: "default",
			Mode: "direct",
		},
		BootType: model.BootType_BOOT_FILE,
		Label: map[string]string{
			"showVnc": "yes",
			"tenant":  "chen",
		},
		Disk: &model.VDisk{
			SystemDisk: &model.VmDisk{
				Name: "bootVol",
				Size: 10737418240,
			},
		},
		Region: "10.124.142.225",
	}
	data, _ := json.Marshal(vmTest)
	fmt.Println(string(data[:]))
	xmlString, err := domainXml(vmTest, ctx)

	if err != nil {
		fmt.Println(err)
		t.Error(err)
	}

	fmt.Println(xmlString)
	t.Log("Testing: Creating domain xml success!")
}
