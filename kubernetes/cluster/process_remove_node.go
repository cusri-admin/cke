package cluster

import (
	"bytes"
	"cke/kubernetes/cluster/conf"
)

//CustomizeRemoveNode remove node 定制化
func CustomizeRemoveNode(cfg *conf.ProcessConf, p *Process, kubeNode *KubeNode, c *Cluster, args ...interface{}) error {
	deletedNodes := make(map[string]*KubeNode)
	if args != nil && len(args) > 0 {
		deletedNodes = args[0].(map[string]*KubeNode)
	}
	ips := bytes.NewBuffer([]byte{})
	names := bytes.NewBuffer([]byte{})
	for _, node := range deletedNodes {
		ips.WriteString(node.NodeIP)
		ips.WriteString(",")
		names.WriteString(node.Name)
		names.WriteString(",")
	}

	nodeNames := names.String()
	nodeIps := ips.String()
	if names.Len() > 0 {
		//去除最后一个","号
		nodeNames = nodeNames[:len(nodeNames)-1]
		nodeIps = nodeIps[:len(nodeIps)-1]
	}
	cfg.SetArg("-i", nodeIps)
	cfg.SetArg("-n", nodeNames)
	return nil
}

// //集群中用于删除节点时清除node痕迹的process
// func GenerateRemoveNodeK8sTask(p *Process, cluster *Cluster, kubeNode *KubeNode, args ...interface{}) (*kubernetes.K8STask, error) {
// 	deletedNodes := make(map[string]*KubeNode)
// 	if args != nil && len(args) > 0 {
// 		deletedNodes = args[0].(map[string]*KubeNode)
// 	}
// 	ips := bytes.NewBuffer([]byte{})
// 	names := bytes.NewBuffer([]byte{})
// 	for _, node := range deletedNodes {
// 		ips.WriteString(node.NodeIp)
// 		ips.WriteString(",")
// 		names.WriteString(node.Name)
// 		names.WriteString(",")
// 	}

// 	nodeNames := names.String()
// 	nodeIps := ips.String()
// 	if names.Len() > 0 {
// 		//去除最后一个","号
// 		nodeNames = nodeNames[:len(nodeNames)-1]
// 		nodeIps = nodeIps[:len(nodeIps)-1]
// 	}
// 	options := []string{"-i", nodeIps, "-n", nodeNames}
// 	p.Options = options
// 	k8sTask := &kubernetes.K8STask{
// 		NodeId:   cluster.ClusterName + "-" + kubeNode.Name + "-container",
// 		TaskType: kubernetes.K8STaskType_START_PROCESS,
// 		Process: &kubernetes.ProcessInfo{
// 			Cmd:  "/usr/local/bin/destory-kubenode.sh",
// 			Args: options,
// 		},
// 	}
// 	return k8sTask, nil
// }
