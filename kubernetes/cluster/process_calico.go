package cluster

import (
	"cke/kubernetes/cluster/conf"
)

//CustomizeCalico Calico定制化
func CustomizeCalico(cfg *conf.ProcessConf, p *Process, kubeNode *KubeNode, c *Cluster, args ...interface{}) error {
	cfg.SetArg("-n", kubeNode.Name)
	return nil
}
