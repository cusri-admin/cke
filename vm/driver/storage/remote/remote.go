package remote

import (
    "bytes"
    "cke/log"
    "cke/vm/image"
    "cke/vm/model"
    "encoding/json"
    "encoding/xml"
    "github.com/libvirt/libvirt-go"
    "io/ioutil"
    "net/http"
)

/*****************
  Author: Chen
  Date: 2019/8/9
  Comp: ChinaUnicom
 ****************/
type DiskDriver struct {
    Name string
    Type string
    Conn *libvirt.Connect
}

var (
    CBS_URL = "https://cbs.console.tg.unicom.local/cbs"
    httpClient  = &http.Client{}
)

func (d *DiskDriver) CreateSystemVolume(vm *model.LibVm) (*map[string]string, error) {
    disk := map[string]string{}

    diskImgFormat := "raw"
    vol, err := d.CreateCBSVolume(vm.Label["user"], "capacity", vm.Region, vm.Name+"-bootVol", vm.Label["poolName"], vm.Disk.SystemDisk.Size)
    if err != nil {
        log.Errorf("Create Boot Volume error occurs. %s", err)
        return nil, err
    }
    bootVolName := "welkin/" + vol.imageName

    resStatus, err := image.CreateBootImg(vm.OsInfo["imgPath"], "rbd:"+bootVolName, diskImgFormat)
    if err != nil || !resStatus {
        log.Errorf("copy image to volume error: %s", err)
        return nil, err
    }
    //TODO 获取资源ID
    if vm.Disk.SystemDisk.Info == nil {
        vm.Disk.SystemDisk.Info = map[string]string{}
    }
    vm.Disk.SystemDisk.Info["volId"] = vol.instanceId
    vm.Disk.SystemDisk.Info["diskImgFormat"] = diskImgFormat
    vm.Disk.SystemDisk.Name = vol.imageName
    vm.Disk.SystemDisk.Size = vol.imageSize

    disk["format"] = diskImgFormat
    disk["path"] = "rbd:" + bootVolName
    return &disk, nil
}

func (d *DiskDriver) CreateMetaVolume (mdata *image.Metadata, udata *string) (string, error){
    /*metaData, err := image.GetMetaTemplate()
    if err != nil {
        log.Errorf("Meta file parse error : %s", err)
    }*/
    return "", nil
    // 创建 NoCloud ISO

}

// Secret创建
func (d *DiskDriver) CreateCephSecret(uid string, keyRing string) (*libvirt.Secret, error) {
    var err error
    var secret *libvirt.Secret

    xmlConfig := &virtSecret{
        Ephemeral: "no", // 是否是临时Secret
        Private:   "no",
        Usage: &virtUsage{
            Type: "ceph", // 指定磁盘类型
            Name: uid,    // secret名称
        },
    }

    xmlSecret, err := xml.Marshal(xmlConfig)
    secret, err = d.Conn.SecretDefineXML(string(xmlSecret), 0)
    if err != nil {
        return nil, err
    }

    // TODO: 设置Secret值
    err = secret.SetValue([]byte(keyRing), 0)
    if err != nil {
        return nil, err
    }

    return secret, nil
}

// Secret删除
func (d *DiskDriver) DeleteCephSecret() error {
    var uuid string
    secret, err := d.Conn.LookupSecretByUUIDString(uuid)
    if err != nil {
        return err
    }
    return secret.Undefine()
}

// Volume创建
func (d *DiskDriver) CreateCBSVolume(user string, cbsType string, region string, name string, poolName string, size int64) (*cbsInstance, error) {
    // /cbs/regions/{regionId}/cbsInstances
    // {
    //  "imageName": "welkinportaltest",
    //  "imageSize": 500,
    //  "poolName": "welkin1d8c16d7",
    //  "type": "capacity"
    //}
    volInfo := &map[string]interface{}{
        "imageName": name,
        "imageSize": size,
        "poolName":  poolName,
        "type":      cbsType,
    }
    b, _ := json.Marshal(volInfo)
    req, err := http.NewRequest(http.MethodPost, CBS_URL+"/regions/"+region+"/cbsInstances", bytes.NewBuffer(b))
    req.Header.Set("Content-Type", "application/json;charset=utf-8")
    req.Header.Set("Cookie", "accessToken=" + user)
    resp, err := httpClient.Do(req)
    defer resp.Body.Close()
    if err != nil {
        log.Errorf("Fetch Storage Pool error. %s", err)
        return nil, err
    }

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        log.Errorf("Parse Create CBS Volume Response Body error. %s", err)
        return nil, err
    }
    var restResp cbsRestResponse
    json.Unmarshal(body, &restResp)

    var volume *cbsInstance
    json.Unmarshal(restResp.Data, &volume)
    return volume, nil
}

// Volume删除
func (d *DiskDriver) DeleteCBSVolume(user string, region string, instanceId string) error {
    // /cbs/regions/{regionId}/cbsInstances/{instanceId}
    req, err := http.NewRequest(http.MethodDelete, CBS_URL+"/regions/"+region+"/cbsInstances/"+instanceId, nil)
    req.Header.Set("Content-Type", "application/json;charset=utf-8")
    req.Header.Set("Cookie", "accessToken=" + user)
    resp, err := httpClient.Do(req)
    defer resp.Body.Close()
    if err != nil || resp.StatusCode/100 != 2 {
        resBody, _ := ioutil.ReadAll(resp.Body)
        log.Errorf("Delete CBS Volume error. %s, %s", err, string(resBody))
        return err
    }
    return nil
}

type virtSecret struct {
    XMLName   xml.Name   `xml:"secret"`
    Ephemeral string     `xml:"ephemeral,attr"`
    Private   string     `xml:"private,attr"`
    Usage     *virtUsage `xml:"usage"`
}

type virtUsage struct {
    Type string `xml:"type,attr"`
    Name string `xml:"name"`
}

//{
//  "code": "OK",
//  "data": {
//    "accountId": "694159455418",
//    "instanceId": "welkin6819772b.test34",
//    "imageName": "test34",
//    "regionId": "xx-tst",
//    "createTime": 1568604122492,
//    "imageSize": 1024,
//    "type": "availability",
//    "userName": "leo",
//    "userId": "694159455418",
//    "imageUsedSize": 0,
//    "poolName": "welkin6819772b",
//    "status": "INITED"
//  }

type cbsRestResponse struct {
    Code string          `json:"code"`
    Data json.RawMessage `json:"data"`
}

type cbsInstance struct {
    accountId     string `json:"accountId"`
    instanceId    string `json:"instanceId"`
    imageName     string `json:"imageName"`
    regionId      string `json:"regionId"`
    createTime    string `json:"createTime"`
    imageSize     int64  `json:"imageSize"`
    cbsType       string `json:"type"`
    username      string `json:"username"`
    userId        string `json:"userId"`
    imageUsedSize int64  `json:"imageUsedSize"`
    pollName      string `json:"pollName"`
    status        string `json:"status"`
}
