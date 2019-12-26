package kubernetes

import (
	cc "cke/cluster"
	kc "cke/kubernetes/cluster"
)

//ProcessItem K8s集群中“进程”的结构体
type ProcessItem struct {
	Name       string          `json:"name"`
	Type       kc.ProcessType   `json:"type"`
	Res        *cc.Resource    `json:"res"`
	Envs       []string        `json:"envs"`
	Options    []string        `json:"options"`
	Cmd        string          `json:"cmd"`
	Attributes []*kc.Attribute `json:"attributes"`
	Expect     string          `json:"expect"`
}


func buildProcessItem(proc *kc.Process) *ProcessItem {
	return &ProcessItem{
		Name:        proc.Name,
		Type:        proc.Type,
		Res:         proc.Res,
		Envs:        proc.Envs,
		Options:     proc.Options,
		Attributes:  proc.Attributes,
		Cmd:		 proc.Cmd,
		Expect:     "RUN",
	}
}

func buildProcessItems(node *kc.KubeNode) []*ProcessItem {
	items := make([]*ProcessItem, 0, 10)
	//TODO fix bug
	/*for _, n := range node.Processes {
		if n.IsNotAdditionalProc() {
			items = append(items, buildProcessItem(n))
		}
	}*/
	return items
}
