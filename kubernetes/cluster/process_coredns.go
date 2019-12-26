package cluster

// func GenerateCoreDnsK8sTask(p *Process, cluster *Cluster, kubeNode *KubeNode, args ...interface{}) (*kubernetes.K8STask, error) {
// 	var apiServers bytes.Buffer
// 	for idx, ip := range cluster.shareData.masterIPs.Values() {
// 		apiServers.WriteString(ip)
// 		if idx < cluster.shareData.masterIPs.Length()-1 {
// 			apiServers.WriteString(",")
// 		}
// 	}
// 	options := []string{"-i", apiServers.String()}
// 	p.Options = options

// 	k8sTask := &kubernetes.K8STask{
// 		NodeId:   cluster.ClusterName + "-" + kubeNode.Name + "-container",
// 		TaskType: kubernetes.K8STaskType_START_PROCESS,
// 		Process: &kubernetes.ProcessInfo{
// 			Cmd:  "/usr/local/bin/init-coredns.sh",
// 			Args: options,
// 		},
// 	}
// 	return k8sTask, nil
// }
