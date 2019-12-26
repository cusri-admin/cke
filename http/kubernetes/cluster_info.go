package kubernetes

import (
	"time"

	cc "cke/cluster"
	kc "cke/kubernetes/cluster"
)

//ClusterInfo 描述Cluster当前状态的struct
type ClusterInfo struct {
	K8sVer     string          `json:"k8s_ver"`
	Name       string          `json:"name"`
	Expect     cc.ExpectStatus `json:"expect"`               //集群目标状态
	Status     Status          `json:"status"`               //集群状态
	Network    string          `json:"network"`              //集群使用的contiv网络名称
	Nodes      []*NodeItem     `json:"nodes"`                //集群中各节点
	//Ingresses  kc.Ingresses    `json:"ingresses"`            //集群的ingress描述
	Ingress   *kc.Ingress      `json:"ingress"`            //集群的ingress描述
	Attributes []*kc.Attribute `json:"attributes,omitempty"` //集群的扩展属性
	CreateTime time.Time       `json:"create_time"`          //集群的创建时间
	Console string          	`json:"console"`          //集群的Kubectl入口
	Dashboard  string          `json:"dashboard"`            //集群Dashboard入口
	Apiserver	string 			`json:"apiserver"`   		 //Apiserver url
	Res        *cc.Resource    `json:"total_res"`            //集群应该使用的资源量
}

func getClusterStatus(cluster *kc.Cluster) Status {
	status := RunningStatus
	for _, n := range cluster.KubeNodes {
		if getNodeStatus(n) != RunningStatus {
			status = StartingStatus
		}
	}
	return status
}

func buildClusterInfo(cluster *kc.Cluster) *ClusterInfo {
	nodes := buildNodeItems(cluster)
	status := getClusterStatus(cluster)
	ci := &ClusterInfo{
		K8sVer:     cluster.K8sVer,
		Name:       cluster.ClusterName,
		Expect:     cluster.Expect,
		Status:     status,
		Network:    cluster.Network,
		Nodes:      nodes,
		//Ingresses:  cluster.Ingresses,
		Ingress:    cluster.Ingress,
		Attributes: cluster.Attributes,
		CreateTime: cluster.CreateTime,
		Res:        cluster.GetResource(),
	}
	ci.Dashboard, _ = cluster.GetDashboardIpPort()
	ci.Apiserver, _= cluster.GetApiServerIpPort()
	ci.Console, _ = cluster.GetGottyIpPort()
	return ci
}

//ClusterMessage 描述Cluster当前状态的struct
type ClusterMessage struct {
	Name       string          `json:"name"`
	Expect     string          `json:"expect"`               //集群目标状态
	Network    string          `json:"network"`              //集群使用的contiv网络名称
	Nodes      []*NodeMsg      `json:"nodes"`                //集群中各节点
	Ingresses  []*IngressItem  `json:"ingresses"`            //集群的ingress描述
	Attributes []*kc.Attribute `json:"attributes,omitempty"` //集群的扩展属性
}

func buildClusterMessage(cluster *kc.Cluster) *ClusterMessage {
	nodes := buildNodeMsgs(cluster)
	//ingresses := buildIngressItems(cluster.Ingresses)
	cm := &ClusterMessage{
		Name:       cluster.ClusterName,
		Expect:     "RUN",
		Network:    cluster.Network,
		Nodes:      nodes,
		//Ingresses:  ingresses,  TODO fix bug
		Attributes: cluster.Attributes,
	}
	return cm
}
