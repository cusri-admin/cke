package cluster

import (
	"cke/kubernetes/cluster/conf"
)

//CustomizeEtcdHc 定制
func CustomizeEtcdHc(cfg *conf.ProcessConf, p *Process, kubeNode *KubeNode, c *Cluster, args ...interface{}) error {
	//TODO: 用变量替换掉硬代码
	tokens := c.tokens["bootstrapToken"] + "," + c.tokens["adminToken"]
	cfg.SetArg("-t", tokens)
	return nil
}
