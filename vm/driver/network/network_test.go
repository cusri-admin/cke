package network

import "testing"

/*****************
  Author: Chen
  Date: 2019/4/23
  Comp: ChinaUnicom
 ****************/
func TestGenerateRandomId(t *testing.T) {
	randomId := GenerateRandomId()
	if randomId == "" || len(randomId) != 8 {
		t.Error("Generating Random id error.")
	}
}

func TestCreateNetworkNamespace(t *testing.T) {
	path := defaultPrefix + "/netns/netfaketest"
	defer RemoveNetworkNamespace(path)

	err := CreateNetworkNamespace(path, false)
	if err != nil {
		t.Fatalf("creating faketest net ns error, %s", err)
	} else {
		t.Logf("creating faketest net ns success.")
	}

	// 获取ns下的信息
}
