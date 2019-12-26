package driver

import (
	netplug "cke/vm/driver/network"
	"cke/vm/driver/network/container"
	"cke/vm/model"
	"flag"
	"testing"
)

func initContext() netplug.Network {
	context := &netplug.NetworkContext{Drivers: make(map[string]netplug.Network)}
	container.Init(context, nil)
	d := context.Drivers[netplug.CONTIV]
	return d
}

func TestCreateDeleteBridge(t *testing.T) {
	d := initContext()
	if d == nil {
		t.Error("Cannot init fake context")
		return
	}

	brName := "br-vm-fake"
	err := d.CreateBridge(brName)

	defer func() {
		err = d.DeleteBridge(brName)
		if err != nil {
			t.Fatalf("Delete Bridge Error, %s", err)
		}
	}()

	if err != nil {
		t.Fatalf("Create Bridge error: %s", err)
	}
	t.Logf("Create bridge success: %s", brName)
}

func TestCreateDeleteEndpoint(t *testing.T) {
	d := initContext()
	if d == nil {
		t.Error("Cannot init fake context")
	}
	epName := flag.Arg(0)
	if epName == "" {
		epName = "ep-vm-5dbbf8c1"
	}
	info, _ := d.CreateNetwork(&model.NetInfo{Name: "net1-wangchen-vpc47403cke"})
	//res, err := d.CreateEndpoint(info, epName)
	defer func() {
		dd := d.(*container.NetDriver)
		dd.LeaveEndpoint(info.ID, epName)
		ep := &netplug.Endpoint{
			Id: epName,
		}
		dd.DeleteEndpoint(info, ep)
	}()
	//if err != nil {
	//   t.Errorf("Res error, %s", err)
	//}
	// t.Logf("Create bridge res: %s", res)
}

func TestBindUnBindDev(t *testing.T) {
	d := initContext()
	if d == nil {
		t.Error("Cannot init fake context")
	}

	random := "fake"

	brName := "br-vm-" + random

	err := d.CreateBridge(brName)
	if err != nil {
		t.Errorf("Creating bridge error, %s", err)
	}

	epName := "ep-vm-" + random
	info, _ := d.CreateNetwork(&model.NetInfo{Name: "spidernet-vpc47403cke"})
	ep, err := d.CreateEndpoint(info, epName)
	if err != nil {
		t.Errorf("Creating endpoint error, %s", err)
	}

	infName, err := d.BindInterfaceToBridge(brName, info.ID, epName)
	if err != nil {
		t.Errorf("Bind endpoint error, %s", err)
	}

	defer func() {
		d.UnBindInterfaceFromBridge(brName, info.ID, epName, infName["name"].(string))
		d.DeleteEndpoint(info, ep)
		d.DeleteBridge(brName)
	}()
}
