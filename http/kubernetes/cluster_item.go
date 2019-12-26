package kubernetes

import (
	"time"

	cc "cke/cluster"
	kc "cke/kubernetes/cluster"
)

//ClusterItem 描述Cluster中Node的简略信息struct
type ClusterItem struct {
	K8sVer      string          `json:"k8s_ver"`
	Name        string          `json:"name"`
	CreateTime  time.Time       `json:"create_time"` //集群的创建时间
	Expect      cc.ExpectStatus `json:"expect"`      //集群目标状态
	Status      Status          `json:"status"`      //集群状态
	Res         *cc.Resource    `json:"total_res"`   //集群使用的资源
	Console     string          `json:"console"` //控制台url
	Dashboard   string          `json:"dashboard"`   //Dashboard url
	Apiserver	string 			`json:"apiserver"`   //Apiserver url
	MasterCount uint16          `json:"master_count"`
	NodeCount   uint16          `json:"node_count"`
}

func buildClusterItem(clr *kc.Cluster) *ClusterItem {
	ci := &ClusterItem{
		K8sVer:      clr.K8sVer,
		Name:        clr.ClusterName,
		CreateTime:  clr.CreateTime,
		MasterCount: 0,
		NodeCount:   0,
		Expect:      clr.Expect,
		Status:      getClusterStatus(clr),
		Res:         clr.GetResource(),
	}
	ci.Dashboard, _ = clr.GetDashboardIpPort()
	ci.Apiserver, _= clr.GetApiServerIpPort()
	ci.Console, _ = clr.GetGottyIpPort()
	for _, n := range clr.KubeNodes {
		if n.Type == kc.KubeNodeType_Master {
			ci.MasterCount++
		} else if n.Type == kc.KubeNodeType_Node {
			ci.NodeCount++
		}
	}
	return ci
}

func buildClusterItems(cls []*kc.Cluster) []*ClusterItem {
	items := make([]*ClusterItem, 0, 10)
	for _, n := range cls {
		items = append(items, buildClusterItem(n))
	}
	return items
}
