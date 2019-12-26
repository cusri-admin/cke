package cluster

import (
	"bytes"
	"cke/kubernetes/cluster/conf"
)

//CustomizeCrBuilding CrBuilding定制化
func CustomizeCrBuilding(cfg *conf.ProcessConf, p *Process, kubeNode *KubeNode, c *Cluster, args ...interface{}) error {
	var nodeIPStr bytes.Buffer
	for idx, nodeIP := range c.shareData.nodeIps.Values() {
		nodeIPStr.WriteString(nodeIP)
		if idx < c.shareData.nodeIps.Length()-1 {
			nodeIPStr.WriteString(",")
		}
	}
	if nodeIPStr.Len() > 0 {
		cfg.SetArg("-i", nodeIPStr.String())
	}
	return nil
}

// func GenerateCrBuildingK8sTask(p *Process, cluster *Cluster, kubeNode *KubeNode, args ...interface{}) (*kubernetes.K8STask, error) {
// 	var nodeIpStr bytes.Buffer
// 	for idx, nodeIp := range cluster.shareData.nodeIps.Values() {
// 		nodeIpStr.WriteString(nodeIp)
// 		if idx < cluster.shareData.nodeIps.Length()-1 {
// 			nodeIpStr.WriteString(",")
// 		}
// 	}
// 	options := IF((nodeIpStr.Len() > 0), []string{"-i", nodeIpStr.String()}, []string{}).([]string)
// 	p.Options = options
// 	k8sTask := &kubernetes.K8STask{
// 		NodeId:   cluster.ClusterName + "-" + kubeNode.Name + "-container",
// 		TaskType: kubernetes.K8STaskType_START_PROCESS,
// 		Process: &kubernetes.ProcessInfo{
// 			Cmd:  "/usr/local/bin/create-node-rolebinding.sh",
// 			Args: options,
// 		},
// 	}
// 	return k8sTask, nil
// }
