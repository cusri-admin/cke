package kubernetes

import (
	"time"

	cc "cke/cluster"
	kc "cke/kubernetes/cluster"
	"fmt"
)

//NodeInfo 描述节点当前状态的struct
type NodeInfo struct {
	Name        string          `json:"name"`
	Type        kc.KubeNodeType `json:"type"`                 //节点类型
	Host        string          `json:"host"`                 //设置的主机
	NodeIP      string          `json:"node_ip"`              //KebeNode使用的IP
	RealHost    string          `json:"real_host"`            //实际运行的主机
	Ingresses   []string        `json:"ingresses"`            //节点开放的ingress
	Attributes  []*kc.Attribute `json:"attributes,omitempty"` //节点的扩展属性
	StartTime   time.Time       `json:"start_time"`           //节点的启动时间
	Status      Status          `json:"status"`               //节点状态
	Expect      cc.ExpectStatus `json:"expect"`               //节点目标状态
	Res         *cc.Resource    `json:"res"`
	Processes   []*ProcInfo     `json:"processes"` //拥有的进程
	Reservation *cc.Reservation `json:"reservation"`
}

func buildNodeIngresses(node *kc.KubeNode) []string {
	ingresses := []string{}
	ingressPorts := node.GetIngresses()
	if ingressPorts != nil {
		for _, port := range node.GetIngresses() {
			ingresses = append(ingresses, fmt.Sprintf("https://%s:%s", node.RealHost, string(port)))
		}
	}
	return ingresses
}

func buildNodeInfo(node *kc.KubeNode) *NodeInfo {
	return &NodeInfo{
		Name:        node.Name,
		Type:        node.Type,
		Host:        node.Host,
		NodeIP:      node.NodeIP,
		RealHost:    node.RealHost,
		Ingresses:   buildNodeIngresses(node),
		Attributes:  node.Attributes,
		StartTime:   node.StartTime,
		Status:      getNodeStatus(node),
		Expect:      node.Expect,
		Res:         node.GetResource(),
		Processes:   buildProcesses(node),
		//Reservation: node.Reservation,
	}
}

func getNodeStatus(node *kc.KubeNode) Status {
	for _, p := range node.Processes {
		if p.Expect == cc.ExpectRUN {
			if p.Task.Status == nil || p.Task.Status.State != cc.TASKMESOS_RUNNING {
				return StartingStatus
			}
		}
	}
	return RunningStatus
}
