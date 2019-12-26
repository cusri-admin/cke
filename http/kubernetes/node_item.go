package kubernetes

import (
	"time"

	cc "cke/cluster"
	kc "cke/kubernetes/cluster"
)

//NodeItem 描述Cluster中Node的简略信息struct
type NodeItem struct {
	Name       string          `json:"name"`
	Type       kc.KubeNodeType `json:"type"`                 //节点类型
	Host       string          `json:"host"`                 //设置的主机
	NodeIP     string          `json:"node_ip"`              //KebeNode使用的IP
	RealHost   string          `json:"real_host"`            //实际运行的主机
	Ingresses  []string        `json:"ingresses,omitempty"`  //节点的ingress描述
	Attributes []*kc.Attribute `json:"attributes,omitempty"` //节点的扩展属性
	StartTime  time.Time       `json:"start_time"`           //节点的创建时间
	Expect     cc.ExpectStatus `json:"expect"`               //节点目标状态
	Status     Status          `json:"status"`               //节点状态
	Res        *cc.Resource    `json:"total_res"`            //节点使用的资源
	//Reservation *cc.Reservation `json:"reservation"`
}

func buildNodeItem(node *kc.KubeNode) *NodeItem {
	return &NodeItem{
		Name:       node.Name,
		Type:       node.Type,
		Host:       node.Host,
		NodeIP:     node.NodeIP,
		RealHost:   node.RealHost,
		Ingresses:  buildNodeIngresses(node),
		Attributes: node.Attributes,
		StartTime:  node.StartTime,
		Status:     getNodeStatus(node),
		Expect:     node.Expect,
		Res:        node.GetResource(),
		//Reservation: node.Reservation,
	}
}

func buildNodeItems(cluster *kc.Cluster) []*NodeItem {
	items := make([]*NodeItem, 0, 10)
	for _, n := range cluster.KubeNodes {
		items = append(items, buildNodeItem(n))
	}
	return items
}

//NodeMsg 描述Cluster中Node的简略信息struct
type NodeMsg struct {
	Name       string          `json:"name"`
	Type       kc.KubeNodeType `json:"type"`                 //节点类型
	Host       string          `json:"host"`                 //设置的主机
	NodeIP     string          `json:"node_ip"`              //KebeNode使用的IP
	Processes  []*ProcessItem  `json:"process"`              //该节点上的运行的系统进程
	Ingresses  []string        `json:"ingresses,omitempty"`  //节点的ingress描述
	Attributes []*kc.Attribute `json:"attributes,omitempty"` //节点的扩展属性
	Expect     string          `json:"expect"`               //节点目标状态

}

func buildNodeMsg(node *kc.KubeNode) *NodeMsg {
	procs := buildProcessItems(node)
	return &NodeMsg{
		Name:      node.Name,
		Type:      node.Type,
		Host:      node.Host,
		NodeIP:    node.NodeIP,
		Processes: procs,
		//Ingresses:  node.Ingresses,  TODO fix bug
		Attributes: node.Attributes,
		Expect:     "RUN",
	}
}

func buildNodeMsgs(cluster *kc.Cluster) []*NodeMsg {
	items := make([]*NodeMsg, 0, 10)
	for _, n := range cluster.KubeNodes {
		items = append(items, buildNodeMsg(n))
	}
	return items
}
