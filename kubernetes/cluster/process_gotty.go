package cluster

import (
	"cke/kubernetes/cluster/conf"
)

//CustomizeGotty Gotty 定制化
func CustomizeGotty(cfg *conf.ProcessConf, p *Process, kubeNode *KubeNode, c *Cluster, args ...interface{}) error {
	return nil
}
