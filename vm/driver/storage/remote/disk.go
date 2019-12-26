package remote

import (
    "bytes"
    "cke/log"
    "encoding/json"
    "errors"
    "io/ioutil"
    "net/http"
    "time"
)

/**
  vm存储磁盘管理
  > 优先使用共享存储，本地存储做测试使用
  > 和天宫平台结合，实现存储资源的统一管理

      获取租户资源池
         +
 +-------+NO
 |       |
YES      v
 |    创建租户资源池
 |       +
 +-------|
         v
      创建BOOT Volume
         +
         |
         v
      创建Data Volume
         +
         |
         v
      根据调度的宿主机绑定磁盘
       格式化磁盘
*/
var (
    DCOSURL      = "http://10.124.142.222"
    TOKEN_KEY    = "dcos-acs-auth-cookie"
    TOKEN_VALUE  = "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiIsImtpZCI6InNlY3JldCJ9.eyJhdWQiOiIzMTk0NDMiLCJpc3MiOiJodHRwOi8vMTAuMTI0LjE0Mi4yMjI6ODEwMiIsImVtYWlsIjoieGlhb3cxMEBjaGluYXVuaWNvbS5jbiIsInN1YiI6IjEiLCJ1aWQiOiJ4aWFvdzEwQGNoaW5hdW5pY29tLmNuIiwiZW1haWxfdmVyaWZpZWQiOnRydWUsIm5vbmNlIjoicG9zdG1hbnRlc3QyIn0.l7ydEPCeEKPkE616bhhiTZO5KFKKDelr2PP6UxZ1QPw"
    TENANT_KEY   = "dcos-acs-tenant-cookie"
    TENENT_VALUE = ""
    CEPHPRE      = "/newapi/storage/ceph"
    client       = &http.Client{}
)

func listVolumePool() ([]*volumePool, error) {
    req, err := http.NewRequest(http.MethodGet, DCOSURL+CEPHPRE+"/stores", nil)
    req.Header.Set("Cookie", TOKEN_KEY+"="+TOKEN_VALUE+";"+TENANT_KEY+"="+TENENT_VALUE+";")
    resp, err := client.Do(req)
    defer resp.Body.Close()
    if err != nil {
        log.Errorf("Fetch Storage Pool error. %s", err)
        return nil, err
    }
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        log.Errorf("Parse Storage Pool Response Body error. %s", err)
        return nil, err
    }
    var restResp restResponse
    json.Unmarshal(body, &restResp)
    var volumePools []*volumePool
    json.Unmarshal(restResp.Data, &volumePools)
    return volumePools, nil
}

func createVolumePool() error {

    return nil
}

func copyVolume() {
}

func createVolume(name string, poolName string, tenant string, size int64, labels []interface{}) (*volume, error) {
    volInfo := &map[string]interface{}{
        "volName":    name,
        "store":      poolName,
        "tenant":     tenant,
        "volSize":    size,
        "labels":     labels,
        "createTime": time.Now().Format("2006-01-02 15:04:05"),
    }
    b, _ := json.Marshal(volInfo)
    req, err := http.NewRequest(http.MethodPost,
        DCOSURL+CEPHPRE+"/tenants/"+tenant+"/volumes",
        bytes.NewBuffer(b))
    req.Header.Set("Cookie", TOKEN_KEY+"="+TOKEN_VALUE+";"+TENANT_KEY+"="+TENENT_VALUE+";")
    req.Header.Set("Content-Type", "application/json;charset=utf-8")
    resp, err := client.Do(req)
    defer resp.Body.Close()
    if err != nil {
        log.Errorf("Fetch Storage Pool error. %s", err)
        return nil, err
    }
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        log.Errorf("Parse Storage Pool Response Body error. %s", err)
        return nil, err
    }
    var restResp restResponse
    json.Unmarshal(body, &restResp)
    if restResp.Status != http.StatusCreated {
        return nil, errors.New("response status code does not match, return: " + restResp.Message)
    }
    var content *volume
    json.Unmarshal(restResp.Data, &content)
    return content, nil
}

func deleteVolume(volId string, tenant string) (bool, error) {
    req, err := http.NewRequest(http.MethodDelete, DCOSURL+CEPHPRE+"/tenants/"+tenant+"/volumes/"+volId, nil)
    req.Header.Set("Cookie", TOKEN_KEY+"="+TOKEN_VALUE+";"+TENANT_KEY+"="+TENENT_VALUE+";")
    resp, err := client.Do(req)
    defer resp.Body.Close()
    if err != nil {
        log.Errorf("Fetch Storage Pool error. %s", err)
        return false, err
    }
    return true, nil
}

func listVolume(poolName string) ([]*volume, error) {
    req, err := http.NewRequest(http.MethodGet,
        DCOSURL+CEPHPRE+"/stores/"+poolName+"/volumes",
        nil)
    req.Header.Set("Cookie", TOKEN_KEY+"="+TOKEN_VALUE+";"+TENANT_KEY+"="+TENENT_VALUE+";")
    req.Header.Set("Content-Type", "application/json;charset=utf-8")
    resp, err := client.Do(req)
    defer resp.Body.Close()
    if err != nil {
        log.Errorf("Fetch Storage Pool error. %s", err)
        return nil, err
    }
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        log.Errorf("Parse Storage Pool Response Body error. %s", err)
        return nil, err
    }
    var restResp restResponse
    json.Unmarshal(body, &restResp)
    var volumes []*volume
    json.Unmarshal(restResp.Data, &volumes)
    return volumes, nil
}

func bindVolume(volId string, hostIp string, tenant string) (bool, error) {
    // (1)Bind IP; CREATED---> VOL_READY
    bindInfo := &map[string]string{
        "volId":  volId,
        "hostIP": hostIp,
        "status": "VOL_READY",
    }
    bindJsonBody, _ := json.Marshal(bindInfo)

    req, err := http.NewRequest(http.MethodPut,
        DCOSURL+CEPHPRE+"/tenants/"+tenant+"/volumes/"+volId,
        bytes.NewBuffer(bindJsonBody))
    req.Header.Set("Cookie", TOKEN_KEY+"="+TOKEN_VALUE+";"+TENANT_KEY+"="+TENENT_VALUE+";")
    req.Header.Set("Content-Type", "application/json;charset=utf-8")
    resp, err := client.Do(req)
    defer resp.Body.Close()
    if err != nil {
        log.Errorf("Fetch Storage Pool error. %s", err)
        return false, err
    }
    return true, nil
}

func unBindVolume(volId string, tenant string) (res bool, err error) {
    //Contains 2 steps
    // (1)UnMount; PUBLISHED---> VOL_READY
    unMountInfo := &map[string]string{
        "volId":  volId,
        "status": "VOL_READY",
    }
    unMountJsonBody, _ := json.Marshal(unMountInfo)
    req, err := http.NewRequest(http.MethodPut,
        DCOSURL+CEPHPRE+"/tenants/"+tenant+"/volumes/"+volId,
        bytes.NewBuffer(unMountJsonBody))
    req.Header.Set("Cookie", TOKEN_KEY+"="+TOKEN_VALUE+";"+TENANT_KEY+"="+TENENT_VALUE+";")
    req.Header.Set("Content-Type", "application/json;charset=utf-8")
    resp, err := client.Do(req)
    defer resp.Body.Close()
    if err != nil {
        log.Errorf("Umount Volume error. %s", err)
        return false, err
    }

    return true, nil
}

func publishVolume(volId string, hostIp string, tenant string, target string, fsType string) (bool, error) {
    // (2)Mount; VOL_READY---> PUBLISHED
    mountInfo := &map[string]string{
        "volId":      volId,
        "targetPath": target,
        "fsType":     fsType,
        "status":     "PUBLISHED",
    }
    mountJsonBody, _ := json.Marshal(mountInfo)
    req, err := http.NewRequest(http.MethodPut,
        DCOSURL+CEPHPRE+"/tenants/"+tenant+"/volumes/"+volId,
        bytes.NewBuffer(mountJsonBody))
    req.Header.Set("Cookie", TOKEN_KEY+"="+TOKEN_VALUE+";"+TENANT_KEY+"="+TENENT_VALUE+";")
    req.Header.Set("Content-Type", "application/json;charset=utf-8")
    resp, err := client.Do(req)
    defer resp.Body.Close()
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        log.Errorf("Parse Storage Pool Response Body error. %s", err)
        return false, err
    }
    var restResp restResponse
    json.Unmarshal(body, &restResp)
    if restResp.Status != http.StatusOK {
        return false, errors.New("response status code does not match, return: " + restResp.Message)
    }
    return true, nil
}

func unPublishVolume(volId string, tenant string) (res bool, err error) {
    // (2)UnPublish; VOL_READY---> CREATED
    unPublishInfo := &map[string]string{
        "volId":  volId,
        "status": "CREATED",
    }
    unPublishJsonBody, _ := json.Marshal(unPublishInfo)
    req, err := http.NewRequest(http.MethodPut,
        DCOSURL+CEPHPRE+"/tenants/"+tenant+"/volumes/"+volId,
        bytes.NewBuffer(unPublishJsonBody))
    req.Header.Set("Cookie", TOKEN_KEY+"="+TOKEN_VALUE+";"+TENANT_KEY+"="+TENENT_VALUE+";")
    req.Header.Set("Content-Type", "application/json;charset=utf-8")
    resp, err := client.Do(req)
    defer resp.Body.Close()
    if err != nil {
        log.Errorf("Unpulish Volume error. %s", err)
        return false, err
    }
    return true, nil
}

type restResponse struct {
    Status  int             `json:"status"`
    Message string          `json:"message"`
    Data    json.RawMessage `json:"data"`
}

type volume struct {
    Tenant       string        `json:"tenant"`
    Id           string        `json:"volId"`
    HostIP       string        `json:"hostIp"`
    Name         string        `json:"volName"`
    PoolName     string        `json:"store"`
    TotalSize    int64         `json:"volSize"`
    UsedSize     int64         `json:"usedSize"`
    RequiredSize int64         `json:"requiredSize"`
    AccessType   string        `json:"accessType"`
    AccessMode   string        `json:"accessMode"`
    TargetPath   string        `json:"targetPath"`
    FsType       string        `json:"fsType"`
    Status       string        `json:"status"`
    Path         string        `json:"devicePath"`
    CreateTime   string        `json:"createTime"`
    DockerId     string        `json:"dockerId"`
    Capability   string        `json:"volumeCapabilities"`
    Label        []interface{} `json:"labels"`
    Permission   bool          `json:"readOnly"`
    ImageFormat  string        `json:"imageFormat"`
    ImageFeature string        `json:"imageFeatures"`
    Mounter      string        `json:"mounter"`
    KeyRing      string        `json:"keyring"`
    Monitors     string        `json:"mounter"`
    UserId       string        `json:"user_id,omitempty"`
    Pool         string        `json:"pool,omitempty"`
}

type volumePool struct {
    PoolName  string `json:"poolName"`
    TotalSize int64  `json:"totalSize"`
    UsedSize  int64  `json:"usedSize"`
    Tenant    string `json:"tenant"`
}
