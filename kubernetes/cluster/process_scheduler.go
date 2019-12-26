package cluster

import (
	"cke/kubernetes/cluster/conf"
)

//CustomizeScheduler Scheduler定制化
func CustomizeScheduler(cfg *conf.ProcessConf, p *Process, kubeNode *KubeNode, c *Cluster, args ...interface{}) error {
	//cfg.SetArg("--master", "http://"+apiServerInsecAddr+":"+apiServerInsecPort)
	return nil
}
