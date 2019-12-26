package cluster

import (
	"cke/kubernetes/cluster/conf"
)

//CustomizeKubeProxy kubeproxy 定制
func CustomizeKubeProxy(cfg *conf.ProcessConf, p *Process, kubeNode *KubeNode, c *Cluster, args ...interface{}) error {
	return nil
}
