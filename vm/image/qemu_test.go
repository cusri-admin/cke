package image

import "testing"

func TestQemuConvertImageToBootVolume(t *testing.T) {
    srcStr := "/data/CentOS-7-x86_64-GenericCloud-1809.qcow2"
    destStr := "/data/instance-test.raw"
    outFormat := "raw"
    res, err := CreateBootImg(srcStr, destStr, outFormat)

    if err != nil {
        t.Fatalf("Error converting.... %s", err)
    }

    t.Log(res)
}
