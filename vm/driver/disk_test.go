package driver

import (
    "cke/vm/image"
    "os"
    "testing"
)

func TestParseMetaData(t *testing.T) {
	metaData, err := image.GetMetaTemplate()
	if err != nil {
		t.Errorf("Meta file parse error : %s", err)
	}

	mdata := image.Metadata{
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
    if f, err := os.Create("meta-data"); err == nil {
        err = metaData.Execute(f, mdata)
        if err != nil {
            t.Error(err)
        }

        err = os.Remove("meta-data")
        if err != nil {
            t.Error(err)
        }
    }
}