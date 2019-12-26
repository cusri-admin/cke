package cluster

import (
	"cke/kubernetes/cluster/conf"
)

//CustomizeController api servercontroller定制化
func CustomizeController(cfg *conf.ProcessConf, p *Process, kubeNode *KubeNode, c *Cluster, args ...interface{}) error {
	return nil
}
