package remote

import (
    "encoding/xml"
    "testing"
)

func TestDiskDriver_CreateCephSecret(t *testing.T) {
    /*d := &DiskDriver{
        Name: "mock-test",
        Type: "remote",
    }
    d.CreateCephSecret();*/
}

func TestXmlMashal(t *testing.T)  {
    xmlConfig := &virtSecret{
        Ephemeral: "no", // 是否是临时Secret
        Private:   "no",
        Usage:     &virtUsage{
            Type: "ceph", // 指定磁盘类型
            Name: "mock-test",    // secret名称
        },
    }

    xmlSecret, err := xml.Marshal(xmlConfig)
    if err != nil {
        t.Error(err)
    }

    t.Log(string(xmlSecret))
}