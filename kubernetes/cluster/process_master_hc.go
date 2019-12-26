package cluster

import (
	"cke/kubernetes/cluster/conf"
)

//CustomizeMasterHc MasterHc定制化
func CustomizeMasterHc(cfg *conf.ProcessConf, p *Process, kubeNode *KubeNode, c *Cluster, args ...interface{}) error {
	//cfg.SetArg("-e", c.shareData.etcdServer.String())
	//cfg.SetArg("-i", *kubeNode.Conf.GetVariable("ClusterPodCidr"))
	if c.Ingress != nil {
		cfg.SetArg("-n", c.Ingress.String())
	}
	return nil
}

/*
func GenerateMasterHcK8sTask(p *Process, cluster *Cluster, kubeNode *KubeNode, args ...interface{}) (*kubernetes.K8STask, error) {
	options := []string{"-e", cluster.shareData.etcdServer.String(), "-i", clusterPodCidr}
	if cluster.Ingress != nil {
		options = append(options, "-n")
		options = append(options, cluster.Ingress.String())
	}
	p.Options = options
	k8sTask := &kubernetes.K8STask{
		NodeId:   cluster.ClusterName + "-" + kubeNode.Name + "-container",
		TaskType: kubernetes.K8STaskType_START_PROCESS,
		Process: &kubernetes.ProcessInfo{
			Cmd:  "/usr/local/bin/k8s-cluster-master-health.sh",
			Args: options,
		},
	}
	return k8sTask, nil
}*/
