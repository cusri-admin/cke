package cluster

import (
	"cke/kubernetes"
	"cke/kubernetes/cluster/conf"
)

//CustomizeNodeConf Node conf定制化
func CustomizeNodeConf(cfg *conf.ProcessConf, p *Process, kubeNode *KubeNode, c *Cluster, args ...interface{}) error {
	cfg.SetArg("-t", c.tokens["bootstrapToken"]+","+c.tokens["adminToken"])
	if lxcfsPath != nil {
		cfg.SetArg("-x", "true")
	}

	cfg.Files["rootKey"] = &kubernetes.K8SFile{
		Path: k8s_root_key_path,
		Mode: 0600,
		Data: c.certs["rootKey"]}
	cfg.Files["rootCrt"] = &kubernetes.K8SFile{
		Path: k8s_root_cert_path,
		Mode: 0644,
		Data: c.certs["rootCrt"]}
	cfg.Files["serverKey"] = &kubernetes.K8SFile{
		Path: k8s_server_key_path,
		Mode: 0600,
		Data: c.certs["serverKey"]}
	cfg.Files["serverCrt"] = &kubernetes.K8SFile{
		Path: k8s_server_cert_path,
		Mode: 0644,
		Data: c.certs["serverCrt"]}
	return nil
}
