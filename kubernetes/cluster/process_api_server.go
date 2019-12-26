package cluster

import (
	"cke/kubernetes/cluster/conf"
	"strconv"
)

//CustomizeAPIServer api server定制化
func CustomizeAPIServer(cfg *conf.ProcessConf, p *Process, kubeNode *KubeNode, c *Cluster, args ...interface{}) error {
	cfg.SetArg("--apiserver-count", strconv.Itoa(c.shareData.masterIPs.Length()))
	return nil
}
