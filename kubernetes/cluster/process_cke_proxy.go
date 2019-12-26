package cluster

import (
	"cke/kubernetes/cluster/conf"
	"cke/kubernetes"
)

//CustomizeGotty Gotty 定制化
func CustomizeCkeProxy(cfg *conf.ProcessConf, p *Process, kubeNode *KubeNode, c *Cluster, args ...interface{}) error {
	//初始化添加cluster ip 网关地址
	for _,node := range c.KubeNodes{
		if node.Type == KubeNodeType_Node {
			cfg.Args = append(cfg.Args, &conf.Parameter{
				Key: "-svc_gateway_node_ip",
				Value: node.NodeIP,
			})
			break
		}
	}
	// //向cke k8s proxy task中增加证书文件
	cfg.Files["serverKey"] = &kubernetes.K8SFile{
		Path: "/etc/kubernetes/ssl/server-key.pem",
		Mode: 0600,
		Data: c.certs["serverKey"]}
	cfg.Files["serverCrt"] = &kubernetes.K8SFile{
		Path: "/etc/kubernetes/ssl/server.pem",
		Mode: 0644,
		Data: c.certs["serverCrt"]}
	return nil
}
