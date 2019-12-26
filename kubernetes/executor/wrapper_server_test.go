package executor

import (
	"testing"

	"cke/kubernetes"
)

type TestKubeNodeManager struct{}

func (t *TestKubeNodeManager) getStartingKubeNode(containerId string) *KubeNodeContainer {
	return &KubeNodeContainer{}
}
func (t *TestKubeNodeManager) moveToRunningKubeNode(containerId string) {
}
func (t *TestKubeNodeManager) getRunningKubeNode(containerId string) *KubeNodeContainer {
	return &KubeNodeContainer{}
}

func TestExec(t *testing.T) {

	execInfo := &kubernetes.ExecInfo{
		NodeId:   "test",
		Cmd:      "sh",
		WorkPath: "",
		Args:     []string{"./wrapper_server_test.sh", "1"},
		Envs:     []string{},
	}
	server := &WrapperServer{
		kubeNodeManager: &TestKubeNodeManager{},
	}
	result, err := server.Exec(nil, execInfo)
	if err != nil {
		t.Errorf("Exec error: %s", err.Error())
	} else if result == nil {
		t.Error("Exec error: result is nil")
	} else {
		t.Logf("(%d)%s", result.Code, result.Result)
	}
}
