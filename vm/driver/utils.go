package driver

import (
    "cke/log"
    "cke/vm/config"
    "cke/vm/driver/network"
    "cke/vm/model"
    "encoding/xml"
    "fmt"
    "github.com/pkg/errors"
    "os/exec"
    "strings"
    "sync"
)

var (
    wg     sync.WaitGroup
    bootVolume *disk
    metaVolume *disk
    dataVolume *disk
    netDev     *networkInfo
    cfg        *config.Config
)

func diskXml(vm *model.LibVm, diskInfo *map[string]string) *disk {
    if vm.BootType == model.BootType_BOOT_FILE {
        fileDisk := &disk{
            Type:   "file",
            Device: "disk",
            Driver: driver{
                Name: "qemu",
                Type: "raw",
            },
            Source: disksrc{
                File: (*diskInfo)["path"],
            },
            Target: disktgt{
                Dev: (*diskInfo)["devName"],
                Bus: "virtio",
            },
        }
        return fileDisk
    }

    // 增加monitor部分
    monitors := make([]monitor, len(cfg.Storage["cephMonitor"].([]string)))
    for _, ip_port := range cfg.Storage["cephMonitor"].([]string) {
        ip_port_s := strings.Split(ip_port, ":")
        m := monitor{
            Name: ip_port_s[0],
            Port: ip_port_s[1],
        }
        monitors = append(monitors, m)
    }

    cephDisk := &disk{
        Type:   "network",
        Device: "storage",
        Driver: driver{
            Name: "qemu",
            Type: "raw",
        },
        Source: disksrc{
            Protocol: "rbd",
            Name:     (*diskInfo)["path"],
            Monitors: monitors,
        },
        Auth: &cephauth{
            UserName: vm.Label["user"],
            Secret: cephsecret{
                Type: "ceph",
                UUID: (*diskInfo)["uuid"],
            },
        },
        Target: disktgt{
            Dev: (*diskInfo)["devName"],
            Bus: "virtio",
        },
    }
    return cephDisk
}

func networkXml(vm *model.LibVm, context *network.NetworkContext) (*networkInfo, error) {
    //Create an interface
    var netInf *networkInfo
    // 基于配置项选择 网络模式
    CreateNetwork(context, vm.Network, nil, vm.Id)
    ep := context.Endpoints[vm.Id]
    if ep == nil {
        // 网络创建失败....
        return nil, errors.New("create network error")
    }
    log.Infof("Creating endpoint success: %s", ep)
    if vm.Network.Mode == "direct" {
        netInf = &networkInfo{
            Type: "direct",
            MacAddr: macAddress{
                Address: ep.MacAddr,
            },
            Source: &netsrc{
                Dev:  ep.EpName,
                Mode: "passthrough",
            },
            Driver: &driver{Name: "vhost"},
            Model:  &netmodel{"virtio"},
            Address: &address{
                Type:     "pci",
                Domain:   "0x0000",
                Bus:      "0x00",
                Slot:     "0x03",
                Function: "0x0",
            },
        }
    } else {
        netInf = &networkInfo{
            Type: "bridge",
            MacAddr: macAddress{
                Address: ep.MacAddr,
            },
            Source: &netsrc{Bridge: ep.BrName},
            Model:  &netmodel{"virtio"},
            Address: &address{
                Type:     "pci",
                Domain:   "0x0000",
                Bus:      "0x00",
                Slot:     "0x03",
                Function: "0x0",
            },
        }
    }
    return netInf, nil
}

func domainXml(vm *model.LibVm, ctx *LibVirtContext) (string, error) {

    cmd, err := exec.LookPath("/usr/libexec/qemu-kvm")
    if err != nil {
        cmd, err = exec.LookPath("qemu-system-x86_64")
        if err != nil {
            return "", fmt.Errorf("cannot find qemu-system-x86_64 binary")
        }
    }

    /*准备网络*/
    wg.Add(1)
    go func() {
        defer wg.Done()
        netDev, err = networkXml(vm, ctx.Network)
        if err != nil {
            return
        }
    }()


    wg.Add(1)
    /*初始化系统盘*/
    go func() {
        defer wg.Done()
        diskInfo, err := CreateSystemDisk(vm, ctx.Storage)
        if err != nil || diskInfo == nil {
            log.Errorf("Create boot disk error occur: %s", err)
        }
        bootVolume = diskXml(vm, diskInfo)
    }()

    /*准备数据磁盘*/
    if len(vm.Disk.DataDisk) > 0 {
        wg.Add(1)
        go func() {
            defer wg.Done()
            diskInfo, _ := CreateDataDisk(vm, ctx.Storage)
            dataVolume = diskXml(vm, diskInfo)
        }()
    }
    wg.Wait() //等待上述设备准备好

    if bootVolume == nil || netDev == nil {
        return "", errors.Errorf("create volume or network error")
    }

    /*初始化元数据盘配置*/
    wg.Add(1)
    go func() {
        defer wg.Done()
        diskInfo, err := CreateMetaDisk(vm, ctx.Network.Endpoints[vm.Id], ctx.Storage)
        if err != nil || diskInfo == nil {
            log.Errorf("Create meta-data disk error occur: %s", err)
        }
        metaVolume = diskXml(vm, diskInfo)
    }()
    wg.Wait()
    //Graphic VNC
    var graph graphics
    if vm.Label["showVnc"] == "yes" {
        graph = graphics{
            Type:     "vnc",
            Port:     "-1",
            AutoPort: "yes",
            Listen:   vm.Region,
            Passwd:   "secret",
            Content: listen{
                Type:    "address",
                Address: vm.Region,
            },
        }
    }

    //Generate a new domain with defined vm parameters
    dom := &domain{
        Type: "kvm",
        Name: vm.Name,
        Memory: memory{
            Unit:    "MiB",
            Content: int(vm.Mem),
        },
        MaxMem: nil,
        VCpu: vcpu{
            Content:   int(vm.Cpus),
            Placement: "static",
        },
        CPU: cpu{
            Mode:  "custom",
            Match: "exact",
            Model: &cpumodel{
                Fallback: "allow",
                Content:  "Haswell-noTSX",
            },
            Numa: nil,
        },
        OS: domainos{
            Type: ostype{
                Arch:    "x86_64",
                Machine: "pc-i440fx-rhel7.0.0",
                Content: "hvm",
            },
        },
        Devices: device{
            Disk:          []*disk{bootVolume, metaVolume, dataVolume},
            NetworkDevice: []*networkInfo{netDev},
            Emulator:      cmd,
            Graphics:      graph,
            Console: console{
                Type: "pty",
                Target: constgt{
                    Type: "serial",
                    Port: "0",
                },
            },
            Memballoon: memballoon{
                Model: "virtio",
                Address: &address{
                    Type:     "pci",
                    Domain:   "0x0000",
                    Bus:      "0x00",
                    Slot:     "0x05",
                    Function: "0x0",
                },
            },
        },
        SecLabel: seclab{
            Type: "none",
        },
    }

    dom.OnPowerOff = "destroy"
    dom.OnReboot = "restart"
    dom.OnCrash = "restart"

    dom.Clock.Offset = "utc"
    dom.Clock.Timer = append(dom.Clock.Timer, timer{Name: "rtc", Track: "guest", Tickpolicy: "catchup"})

    data, err := xml.Marshal(dom)
    if err != nil {
        return "", err
    }
    return string(data), err
}

type memory struct {
    Unit    string `xml:"unit,attr"`
    Content int    `xml:",chardata"`
}

type maxmem struct {
    Unit    string `xml:"unit,attr,omitempty"`
    Slots   string `xml:"slots,attr,omitempty"`
    Content int    `xml:",chardata"`
}

type vcpu struct {
    Placement string `xml:"placement,attr"`
    Current   string `xml:"current,attr,omitempty"`
    Content   int    `xml:",chardata"`
}

type cpumodel struct {
    Fallback string `xml:"fallback,attr"`
    Content  string `xml:",chardata"`
}

type cell struct {
    Id     string `xml:"id,attr"`
    Cpus   string `xml:"cpus,attr"`
    Memory string `xml:"memory,attr"`
    Unit   string `xml:"unit,attr"`
}

type numa struct {
    Cell []cell `xml:"cell"`
}

type cpu struct {
    Mode  string    `xml:"mode,attr"`
    Match string    `xml:"match,attr,omitempty"`
    Model *cpumodel `xml:"model,omitempty"`
    Numa  *numa     `xml:"numa,omitempty"`
}

type ostype struct {
    Arch    string `xml:"arch,attr"`
    Machine string `xml:"machine,attr"`
    Content string `xml:",chardata"`
}

type osloader struct {
    Type     string `xml:"type,attr"`
    ReadOnly string `xml:"readonly,attr"`
    Content  string `xml:",chardata"`
}

type domainos struct {
    Supported string    `xml:"supported,attr,omitempty"`
    Type      ostype    `xml:"type"`
    Kernel    string    `xml:"kernel,omitempty"`
    Initrd    string    `xml:"initrd,omitempty"`
    Cmdline   string    `xml:"cmdline,omitempty"`
    Loader    *osloader `xml:"loader,omitempty"`
    Nvram     string    `xml:"nvram,omitempty"`
}

type features struct {
    Acpi string `xml:"acpi"`
}

type address struct {
    Type       string `xml:"type,attr"`
    Domain     string `xml:"domain,attr,omitempty"`
    Controller string `xml:"controller,attr,omitempty"`
    Bus        string `xml:"bus,attr"`
    Slot       string `xml:"slot,attr,omitempty"`
    Function   string `xml:"function,attr,omitempty"`
    Target     int    `xml:"target,attr,omitempty"`
    Unit       int    `xml:"unit,attr,omitempty"`
}

type controller struct {
    Type    string   `xml:"type,attr"`
    Index   string   `xml:"index,attr,omitempty"`
    Model   string   `xml:"model,attr,omitempty"`
    Address *address `xml:"address,omitempty"`
}

type fsdriver struct {
    Type string `xml:"type,attr"`
}

type fspath struct {
    Dir string `xml:"dir,attr"`
}

type filesystem struct {
    Type       string   `xml:"type,attr"`
    Accessmode string   `xml:"accessmode,attr"`
    Driver     fsdriver `xml:"driver"`
    Source     fspath   `xml:"source"`
    Target     fspath   `xml:"target"`
    Address    *address `xml:"address"`
}

type channsrc struct {
    Mode string `xml:"mode,attr"`
    Path string `xml:"path,attr"`
}

type cephsecret struct {
    Type string `xml:"type,attr"`
    UUID string `xml:"uuid,attr"`
}

type cephauth struct {
    UserName string     `xml:"username,attr"`
    Secret   cephsecret `xml:"secret"`
}

type channtgt struct {
    Type string `xml:"type,attr"`
    Name string `xml:"name,attr"`
}

type channel struct {
    Type   string   `xml:"type,attr"`
    Source channsrc `xml:"source"`
    Target channtgt `xml:"target"`
}

type clock struct {
    Offset string  `xml:"offset,attr"`
    Timer  []timer `xml:"timer,omitempty"`
}

type constgt struct {
    Type string `xml:"type,attr"`
    Port string `xml:"port,attr"`
}

type console struct {
    Type   string   `xml:"type,attr"`
    Source channsrc `xml:"source,omitempty"`
    Target constgt  `xml:"target"`
}

type memballoon struct {
    Model   string   `xml:"model,attr"`
    Address *address `xml:"address"`
}

type device struct {
    Emulator      string         `xml:"emulator"`
    Disk          []*disk        `xml:"disk,omitempty"`
    Controllers   []controller   `xml:"controller,omitempty"`
    Filesystems   []filesystem   `xml:"filesystem,omitempty"`
    Channels      []channel      `xml:"channel,omitempty"`
    Console       console        `xml:"console,omitempty"`
    Memballoon    memballoon     `xml:"memballoon,omitempty"`
    NetworkDevice []*networkInfo `xml:"interface,omitempty"`
    Graphics      graphics       `xml:"graphics,omitempty"`
}

type disk struct {
    Type     string    `xml:"type,attr"`
    Device   string    `xml:"device,attr"`
    Driver   driver    `xml:"driver,omitempty"`
    Source   disksrc   `xml:"source"`
    Target   disktgt   `xml:"target,omitempty"`
    Auth     *cephauth `xml:"auth,omitempty"`
    ReadOnly string    `xml:"readonly,omitempty"`
    Iotune   iotune    `xml:"iotune,omitempty"`
}

type iotune struct {
    BytesPerSec int `xml:"total_bytes_sec,omitempty"`
    Iops        int `xml:"total_iops_sec,omitempty"`
}

type driver struct {
    Name string `xml:"name,attr"`
    Type string `xml:"type,attr"`
}

type graphics struct {
    Type     string `xml:"type,attr"`
    Port     string `xml:"port,attr"`
    AutoPort string `xml:"autoport,attr"`
    Listen   string `xml:"listen,attr"`
    Passwd   string `xml:"passwd,attr"`
    Content  listen `xml:"listen"`
}

type listen struct {
    Type    string `xml:"type,attr"`
    Address string `xml:"address,attr"`
}

type disksrc struct {
    File     string `xml:"file,attr,omitempty"`
    Protocol string `xml:"protocol,attr,omitempty"`
    Name     string `xml:"name,attr,omitempty"`
    Monitors []monitor
}

type disktgt struct {
    Dev  string `xml:"dev,attr,omitempty"`
    File string `xml:"file,attr,omitempty"`
    Bus  string `xml:"bus,attr,omitempty"`
}

type macAddress struct {
    Address string `xml:"address,attr"`
}

type monitor struct {
    XMLName xml.Name `xml:"host"`
    Name    string   `xml:"name,attr"`
    Port    string   `xml:"port,attr"`
}

type networkInfo struct {
    Type    string     `xml:"type,attr"`
    MacAddr macAddress `xml:"mac"`
    Driver  *driver    `xml:"driver"`
    Source  *netsrc    `xml:"source"`
    Model   *netmodel  `xml:"model"`
    Address *address   `xml:"address"`
}

type netmodel struct {
    Type string `xml:"type,attr"`
}

type netsrc struct {
    Bridge string `xml:"bridge,attr"`
    Dev    string `xml:"dev,attr"`
    Mode   string `xml:"mode,attr"`
}

type qemucmd struct {
    Value string `xml:"value,attr"`
}

type seclab struct {
    Type string `xml:"type,attr"`
}

type timer struct {
    Name       string  `xml:"name,attr"`
    Track      string  `xml:"track,attr,omitempty"`
    Tickpolicy string  `xml:"tickpolicy,attr,omitempty"`
    CatchUp    catchup `xml:"catchup,omitempty"`
    Frequency  uint32  `xml:"frequency,attr,omitempty"`
    Mode       string  `xml:"mode,attr,omitempty"`
    Present    string  `xml:"present,attr,omitempty"`
}

type catchup struct {
    Threshold uint `xml:"threshold,attr,omitempty"`
    Slew      uint `xml:"slew,attr,omitempty"`
    Limit     uint `xml:"limit,attr,omitempty"`
}

type commandlines struct {
    Cmds []qemucmd `xml:"qemu:arg"`
}

type domain struct {
    XMLName     xml.Name     `xml:"domain"`
    Type        string       `xml:"type,attr"`
    XmlnsQemu   string       `xml:"xmlns:qemu,attr,omitempty"`
    Name        string       `xml:"name"`
    Memory      memory       `xml:"memory"`
    MaxMem      *maxmem      `xml:"maxMemory,omitempty"`
    VCpu        vcpu         `xml:"vcpu"`
    OS          domainos     `xml:"os"`
    Features    features     `xml:"features"`
    CPU         cpu          `xml:"cpu"`
    OnPowerOff  string       `xml:"on_poweroff"`
    OnReboot    string       `xml:"on_reboot"`
    OnCrash     string       `xml:"on_crash"`
    Devices     device       `xml:"devices"`
    SecLabel    seclab       `xml:"seclabel"`
    Clock       clock        `xml:"clock"`
    CommandLine commandlines `xml:"qemu:commandline,omitempty"`
}
