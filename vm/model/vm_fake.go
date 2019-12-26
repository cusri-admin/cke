package model

type BootType_Fake uint

const (
    BRIDGE_Fake                   = "bridge"
    OVERLAY_Fake                  = "overlay"
    LOCAL_Fake                    = "local"
    BOOT_FILE_Fake  BootType_Fake = 0
    BOOT_SHARE_Fake BootType_Fake = 1
)

type LibVm_Fake struct {
    Id       string                 `json:"id"`
    Name     string                 `json:"name"`
    Cpus     int                    `json:"cpus"`
    Host     string                 `json:"host"`
    Mem      int                    `json:"mem"`
    Os       map[string]interface{} `json:"os"`
    Network  NetInfo_Fake           `json:"network,omitempty"`
    BootType BootType_Fake          `json:"boot_type,uint"`
    Disk     VDisk_Fake             `json:"storage"`
    Region   string                 `json:"region"`
    Label    map[string]interface{} `json:"label"`
}

type VDisk_Fake struct {
    BootVol VmDisk_Fake   `json:"boot_vol"`
    DataVol []VmDisk_Fake `json:"data_vol,omitempty"`
}

type NetInfo_Fake struct {
    Id       string `json:"id"`
    Name     string `json:"name"`
    Type     string `json:"type"`
    Mode     string `json:"mode"`
    Ipv4Addr string `json:"addr4,omitempty"`
    Ipv6Addr string `json:"addr6,omitempty"`
}

type VmDisk_Fake struct {
    Name string                 `json:"name"`
    Type string                 `json:"type"`
    Size int64                  `json:"size"`
    Info map[string]interface{} `json:"info"`
}
