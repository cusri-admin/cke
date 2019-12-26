package cluster

import (
	"bytes"
	"cke/kubernetes"
	"cke/kubernetes/cluster/conf"
	"sort"
	"strconv"
)

//CustomizeEtcd etcd定制化
func CustomizeEtcd(cfg *conf.ProcessConf, p *Process, kubeNode *KubeNode, c *Cluster, args ...interface{}) error {
	curMasterIndex := 0
	var initailClusters bytes.Buffer
	masterIPArray := c.shareData.masterIPs.Values()
	sort.Strings(masterIPArray)
	for idx, masterIP := range masterIPArray {
		if masterIP == kubeNode.NodeIP {
			curMasterIndex = idx
		}
		initailClusters.WriteString("etcd-")
		initailClusters.WriteString(strconv.Itoa(idx))
		initailClusters.WriteString("=https://")
		initailClusters.WriteString(masterIP)
		initailClusters.WriteString(":2390")
		if idx < len(masterIPArray)-1 {
			initailClusters.WriteString(",")
		}
	}
	//配置ETCD name
	cfg.SetArg("--name", "etcd-"+strconv.Itoa(curMasterIndex))
	cfg.SetArg("--initial-cluster", initailClusters.String())

	// //向etcd task中增加证书文件
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
