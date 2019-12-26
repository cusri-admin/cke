package cluster

import (
	"cke/kubernetes"
	"cke/kubernetes/cluster/conf"

	"gopkg.in/yaml.v2"
)

//CustomizeKubelet kubelet 定制化
func CustomizeKubelet(cfg *conf.ProcessConf, p *Process, kubeNode *KubeNode, c *Cluster, args ...interface{}) error {
	//向节点添加是服务提供节点的kubelet node label
	if kubeNode.Ingress {
		if len(cfg.GetArg("--node-labels")) > 0 {
			cfg.SetArg("--node-labels", cfg.GetArg("--node-labels")+","+getNodeLabels())
		} else {
			cfg.SetArg("--node-labels", getNodeLabels())
		}
	}

	//TODO: 问大平为什么需要 process.ConfigJSON
	//2. 判断是否有kubelet的process配置文件，如果有则直接使用，否则读取默认参数(yaml格式)
	if p.ConfigJSON == nil {
		if fileBytes, err := conf.FetchDefaultConfigBytes("kubelet.config"); err == nil {
			var configYalm interface{}
			if err := yaml.Unmarshal(fileBytes, &configYalm); err != nil {
				return err
			}
			p.ConfigJSON = convert(configYalm)
		} else {
			return err
		}
	}
	var corednsIPs []string
	for _, ip := range c.shareData.masterIPs.Values() {
		corednsIPs = append(corednsIPs, ip)
	}

	p.ConfigJSON.(map[string]interface{})["address"] = kubeNode.NodeIP

	var kubeConfigBytes []byte
	var err error
	if kubeConfigBytes, err = yaml.Marshal(p.ConfigJSON); err != nil {
		return err
	}

	cfg.Files["kubelet.config"] = &kubernetes.K8SFile{
		Path: "/etc/kubernetes/cfg/kubelet.config",
		Mode: 600,
		Data: kubeConfigBytes,
	}
	return nil
}

//getNodeLabels 获得配置在kubelet进程中的labels
func getNodeLabels() string {
	return "ingress_" + IngressType_Bridge.String() + "=1"
}
