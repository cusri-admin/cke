package exec

import (
	e "cke/executor"
	"cke/vm/model"
	"testing"
)

/*****************
  Author: Chen
  Date: 2019/8/6
  Comp: ChinaUnicom
 ****************/
type TestCaller struct {
	count int
}

func (tc *TestCaller) CallTaskStarting(string) error       { return nil }
func (tc *TestCaller) CallTaskRunning(string) error        { return nil }
func (tc *TestCaller) CallTaskFinished(string) error       { return nil }
func (tc *TestCaller) CallTaskKilled(string) error         { return nil }
func (tc *TestCaller) CallTaskFailed(string, string) error { return nil }
func (tc *TestCaller) CallMessage(message []byte) error    { return nil }

func TestNewExecutor(t *testing.T) {
	exec, err := NewExecutor()
	if err != nil {
		t.Errorf("Init Executor error : %s", err)
	}
	defer exec.Shutdown()

	task := &e.TaskInfoWithResource{
		Cpus: 0.1,
		Mem:  128,
		Task: &model.LibVm{
			Name:   "instance-test",
			Cpus:   1,
			Mem:    2048,
			Region: "10.124.142.225",
			Os: map[string]string{
				"imgPath": "/data/CentOS-7-x86_64-GenericCloud-1809.qcow2",
			},
			Network: &model.NetInfo{
				Name: "test",
				Type: "default",
				Mode: "direct",
			},
			BootType: model.BootType_BOOT_FILE,
			Disk: &model.VDisk{
				BootVol: &model.VmDisk{
					Name: "bootVol",
					Size: 10737418240,
				},
			},
			Label: map[string]string{
				"showVnc": "yes",
				"tenant":  "chen",
			},
		},
	}

	caller := &TestCaller{}

	exec.Registered(caller)

	err = exec.Launch("TestTaskID", task)
	if err != nil {
		t.Errorf("executor.Launch err: %s", err.Error())
	}
	t.Log(exec.driver)

	defer func() {
		if isAlive, _ := exec.driver.IsAlive(); isAlive {
			err = exec.Kill("TestTaskID")
			if err != nil {
				t.Errorf("executor kill task err: %s", err.Error())
			}
		} else {
			t.Errorf("Executor or driver has existed...")
		}
	}()

}
