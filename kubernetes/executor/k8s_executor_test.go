package executor

import (
	// "bytes"
	// "io"
	// "strings"
	e "cke/executor"
	"cke/kubernetes"
	"testing"
)

type TestCaller struct {
	count int
}

func (tc *TestCaller) CallTaskStarting(string) error       { return nil }
func (tc *TestCaller) CallTaskRunning(string) error        { return nil }
func (tc *TestCaller) CallTaskFinished(string) error       { return nil }
func (tc *TestCaller) CallTaskKilled(string) error         { return nil }
func (tc *TestCaller) CallTaskFailed(string, string) error { return nil }
func (tc *TestCaller) CallMessage(message []byte) error    { return nil }

func TestLaunch(t *testing.T) {
	exec, err := NewExecutor("fwId123456", "/tmp/workPath", "kubeNodeImage", "/tmp/socketPath", false, "", nil)
	if err != nil {
		t.Errorf("NewExecutor err: %s", err.Error())
	} else if exec == nil {
		t.Error("Executor is nil")
	}

	task := &e.TaskInfoWithResource{
		Cpus: 0.1,
		Mem:  128,
		Task: &kubernetes.K8STask{
			NodeId: "test",
		},
	}

	caller := &TestCaller{}

	exec.Registered(caller)

	err = exec.Launch("TestTaskID", task)
	if err != nil {
		t.Errorf("executor.Launch err: %s", err.Error())
	}
}

func TestKill(t *testing.T) {
	exec, err := NewExecutor("fwId123456", "/tmp/workPath", "kubeNodeImage", "/tmp/socketPath", false, "", nil)
	if err != nil {
		t.Errorf("NewExecutor err: %s", err.Error())
	}
	err = exec.Kill("mesosTaskId")
	if err != nil {
		t.Errorf("executor.Launch err: %s", err.Error())
	}
}
