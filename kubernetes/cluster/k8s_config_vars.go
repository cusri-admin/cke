package cluster

func IF(condition bool, trueVal, falseVal interface{}) interface{} {
	if condition {
		return trueVal
	}
	return falseVal
}

//kube node中证书位置
var k8s_root_cert_path string = "/etc/kubernetes/ssl/ca.pem"
var k8s_root_key_path string = "/etc/kubernetes/ssl/ca-key.pem"
var k8s_server_cert_path string = "/etc/kubernetes/ssl/server.pem"
var k8s_server_key_path string = "/etc/kubernetes/ssl/server-key.pem"
var k8s_cert_dir string = "/etc/kubernetes/ssl"
var k8s_registry_cert_dir = "/etc/docker/certs.d"

//var k8s_default_cfg_dir = "/cke-scheduler"

//apiserver security port
var apiServerSecPort = "6443"

//apiserver insecurity address
var apiServerInsecAddr = "127.0.0.1"

//apiserver insecurity port
var apiServerInsecPort = "8081"

//service-cluster-ip-range
var clusterIpRange = "11.254.0.0/16"

//apiserver token file path
var apiServerTokenFile = "/etc/kubernetes/cfg/token.csv"

//cluster pod cidr calico网络
var clusterPodCidr = "172.30.0.0/16"
