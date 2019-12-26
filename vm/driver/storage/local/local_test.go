package local

import (
    "cke/vm/image"
    "testing"
)

func TestDiskDriver_CreateMetaVolume(t *testing.T) {
    mdata := &image.Metadata{
        InstanceId: "1",
        IfName: "eth0",
        Ipv4Addr: "192.168.122.101",
        Ipv4Network: "192.168.122.0",
        Ipv4Mask: "255.255.255.0",
        Gateway: "192.168.122.254",
        Hostname: "vm-1-192-168-122-101",
        Password: "passw0rd",
        IsPwdExpire: false,
    }

    udata := "#"

    d := DiskDriver{
        Name: "local",
        Type: "local",
    }
    res, err := d.CreateMetaVolume(mdata, &udata)
    t.Errorf("%s", err)
    t.Log(res)
}