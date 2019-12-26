package remote

import (
    "testing"
)

func TestCreateVolume(t *testing.T) {
    DCOSURL = "http://10.124.142.222"
    TOKEN_VALUE = "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiIsImtpZCI6InNlY3JldCJ9.eyJhdWQiOiIzMTk0NDMiLCJpc3MiOiJodHRwOi8vMTAuMTI0LjE0Mi4yMjI6ODEwMiIsImVtYWlsIjoieGlhb3cxMEBjaGluYXVuaWNvbS5jbiIsInN1YiI6IjEiLCJ1aWQiOiJ4aWFvdzEwQGNoaW5hdW5pY29tLmNuIiwiZW1haWxfdmVyaWZpZWQiOnRydWUsIm5vbmNlIjoicG9zdG1hbnRlc3QyIn0.l7ydEPCeEKPkE616bhhiTZO5KFKKDelr2PP6UxZ1QPw"
    tenant := "suhui"
    volName := "v4"
    size := int64(10737418240) //10G
    labels := make([]interface{}, 0)

    v, err := createVolume(volName, "welkin-ceph-"+tenant, tenant, size, labels)
    if err != nil {
        t.Fatalf("create volume err, %s", err)
    }

    if v == nil {
        t.Fatalf("Could not return volume info")
    }
    t.Logf("create volume result: %s, %s", v.Name, err)

    /*
       //bind卷
       bindRes, err := bindVolume(res.Name, ip, tenant)
       if err != nil {
           t.Fatalf("bind volume err, %s", err)
       }
       t.Logf("bind volume res: %t", bindRes)

       //publish卷
       pubRes, err := publishVolume(res.Name, ip, tenant, target, fsType)
       if err != nil {
           t.Fatalf("publish volume err, %s", err)
       }
       t.Logf("publish volume res: %t", pubRes)
    */
    //copy volume to boot image
    /*err = copyImageToVolume(imagePath, destPath, "raw")
      if err != nil {
          t.Fatalf("copy image to volume error: %s", err)
      }*/
    /*
       //unpublish卷
       unpubres, err := unPublishVolume(v.(string), tenant)
       if err != nil {
           t.Fatalf("unpublish volume err, %s", err)
       }
       t.Logf("unpublish volume res: %t", unpubres)
    */
    //删除卷
    result, err := deleteVolume(v.Id, tenant)
    if err != nil {
        t.Fatalf("delete volume err, %s", err)
    }
    t.Logf("delete volume result: %t, %s", result, err)

}

func TestListVolume(t *testing.T) {
    DCOSURL = "http://10.124.142.222"
    TOKEN_VALUE = "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiIsImtpZCI6InNlY3JldCJ9.eyJhdWQiOiIzMTk0NDMiLCJpc3MiOiJodHRwOi8vMTAuMTI0LjE0Mi4yMjI6ODEwMiIsImVtYWlsIjoieGlhb3cxMEBjaGluYXVuaWNvbS5jbiIsInN1YiI6IjEiLCJ1aWQiOiJ4aWFvdzEwQGNoaW5hdW5pY29tLmNuIiwiZW1haWxfdmVyaWZpZWQiOnRydWUsIm5vbmNlIjoicG9zdG1hbnRlc3QyIn0.l7ydEPCeEKPkE616bhhiTZO5KFKKDelr2PP6UxZ1QPw"
    tenant := "t1"
    _, err := listVolume("welkin-ceph-" + tenant)
    if err != nil {
        t.Fatal(err)
    }
}

func TestListPool(t *testing.T) {
    DCOSURL = "http://10.124.142.222"
    TOKEN_VALUE = "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiIsImtpZCI6InNlY3JldCJ9.eyJhdWQiOiIzMTk0NDMiLCJpc3MiOiJodHRwOi8vMTAuMTI0LjE0Mi4yMjI6ODEwMiIsImVtYWlsIjoieGlhb3cxMEBjaGluYXVuaWNvbS5jbiIsInN1YiI6IjEiLCJ1aWQiOiJ4aWFvdzEwQGNoaW5hdW5pY29tLmNuIiwiZW1haWxfdmVyaWZpZWQiOnRydWUsIm5vbmNlIjoicG9zdG1hbnRlc3QyIn0.l7ydEPCeEKPkE616bhhiTZO5KFKKDelr2PP6UxZ1QPw"
    res, err := listVolumePool()

    if err != nil {
        t.Fatalf("list volume pool err")
    }

    t.Logf("current volume pool size: %d", len(res))

    for _, vol := range res {
        t.Logf(vol.PoolName)
    }
}
