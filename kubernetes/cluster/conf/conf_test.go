package conf

import (
	"testing"
)

//go test -v cke/kubernetes/cluster/conf -args -k8s_config cke/build/scheduler/default_cfg/cke.conf.yaml

func TestReadCKEConf(t *testing.T) {
	cfg, err := LoadCKEConf()
	if err != nil {
		t.Errorf("TestReadProcessConf error: %v\n", err)
	}
	cfg.PrintCKEConf()
}
