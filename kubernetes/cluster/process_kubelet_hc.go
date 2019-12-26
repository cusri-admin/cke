package cluster

import (
	"cke/kubernetes/cluster/conf"
)

//CustomizeKubeletHc 定制化集群中为kubelet做approve及健康检查的process
func CustomizeKubeletHc(cfg *conf.ProcessConf, p *Process, kubeNode *KubeNode, c *Cluster, args ...interface{}) error {
	//cfg.SetArg("-i", kubeNode.NodeIp)
	return nil
}
