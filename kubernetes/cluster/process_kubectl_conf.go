package cluster

import (
	"cke/kubernetes/cluster/conf"
)

//CustomizeKubectlConf Kubectl Conf 定制化
func CustomizeKubectlConf(cfg *conf.ProcessConf, p *Process, kubeNode *KubeNode, c *Cluster, args ...interface{}) error {
	cfg.SetArg("-t", c.tokens["adminToken"])
	return nil
}
