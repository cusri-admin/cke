package cluster

import (
	"cke/kubernetes"
	"cke/kubernetes/cluster/conf"
	"strings"
)

//CustomizeDocker Docker定制化
func CustomizeDocker(cfg *conf.ProcessConf, p *Process, kubeNode *KubeNode, c *Cluster, args ...interface{}) error {
	certName := (*registryCert)[strings.LastIndex(*registryCert, "/")+1:]
	certDir := certName[:strings.LastIndex(certName, ".")]
	cfg.Files["k8s_registry_cert"] = &kubernetes.K8SFile{
		Path: k8s_registry_cert_dir + "/" + certDir + "/" + certName,
		Mode: 0644,
		Data: c.certs["registryCert"]}
	return nil
}
