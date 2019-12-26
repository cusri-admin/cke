package image

import "testing"

/*****************
  Author: Chen
  Date: 2019/3/8
  Comp: ChinaUnicom
 ****************/

func TestGetMetaTemplate(t *testing.T) {
    _, err := GetMetaTemplate()
    if err != nil {
        t.Error(err)
    }
}

func TestCreateMetaImage(t *testing.T) {
    err := CreateMetaImage("/tmp/meta-data", "/tmp/user-data", "/tmp/seed1.iso")
    if err != nil {
        t.Errorf("%s", err)
    }
}