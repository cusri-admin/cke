package kubernetes

import (
	"time"

	cc "cke/cluster"
	kc "cke/kubernetes/cluster"
)

//ProcInfo 描述进程当前状态的struct
type ProcInfo struct {
	Name       string          `json:"name"`
	Type       kc.ProcessType  `json:"type"`
	Status     Status          `json:"status"`               //进程状态
	Res        *cc.Resource    `json:"res"`                  //进程使用的资源
	Envs       []string        `json:"envs"`                 //进程的环境变量
	Cmd        string          `json:"cmd"`                  //进程的命令行
	Options    []string        `json:"options"`              //进程的命令行参数
	Attributes []*kc.Attribute `json:"attributes,omitempty"` //进程的扩展属性
	StartTime  time.Time       `json:"start_time"`           //进程的启动时间
	Expect     cc.ExpectStatus `json:"expect"`               //进程的期望状态
}

func buildProcInfo(proc *kc.Process) *ProcInfo {
	return &ProcInfo{
		Name:   proc.Name,
		Type:   proc.Type,
		Status: getProcStatus(proc),
		Res:    proc.Res,
		//Envs:       proc.Env,
		Cmd:        proc.Cmd,
		Options:    proc.Options,
		Attributes: proc.Attributes,
		StartTime:  proc.StartTime,
		Expect:     proc.Expect,
	}
}

func getProcStatus(proc *kc.Process) Status {
	if proc.Task.Status != nil && proc.Task.Status.State == cc.TASKMESOS_RUNNING {
		return RunningStatus
	}
	return StartingStatus
}

func buildProcesses(node *kc.KubeNode) []*ProcInfo {
	items := make([]*ProcInfo, 0, 10)
	for _, p := range node.Processes {
		if p.Expect == cc.ExpectRUN {
			items = append(items, buildProcInfo(p))
		}
	}
	return items
}
