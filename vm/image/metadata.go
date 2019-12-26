package image

import (
    "cke/log"
    "html/template"
    "os/exec"
)

/*****************
  Author: Chen
  Date: 2019/2/26
  Comp: ChinaUnicom
 ****************/
type Metadata struct {
    InstanceId    string
    IfName        string
    Ipv4Addr      string
    Ipv4Network   string
    Ipv4Mask      string
    Ipv4Broadcast string
    Gateway       string
    Hostname      string
    Password      string
    IsPwdExpire   bool
}

var tmpl = `instance-id: {{.InstanceId}}
local-hostname: {{.Hostname}}
network-interfaces: |
  iface {{.IfName}} inet static
  address {{.Ipv4Addr}}
  network {{.Ipv4Network}}
  netmask {{.Ipv4Mask}}
  gateway {{.Gateway}}
  {{with .Ipv4Broadcast}}broadcast {{.}}{{end}}`

//镜像元数据管理，用于启动镜像时的配置
func GetMetaTemplate() (*template.Template, error){
    return template.New("meta-data").Parse(tmpl)
}


func CreateMetaImage(metadata string, userdata string, destPath string) (error) {
    // genisoimage  -output seed.iso -volid cidata -joliet -rock user-data meta-data

    cmd := exec.Command("/bin/genisoimage", "-output", destPath, "-volid", "cidata", "-joliet", "-rock", metadata, userdata)
    err := cmd.Run()
    if err != nil {
        log.Errorf("create meta image error occured.")
        return err
    }
    return nil
}

func RemoveMetaImage()  {

}