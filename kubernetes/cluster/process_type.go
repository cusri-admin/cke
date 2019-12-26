package cluster

import (
	"cke/cluster"
	"cke/kubernetes/cluster/conf"
)

//CustomizeTask 对进程的任务进行定制化处理的函数定义
type CustomizeTask func(cfg *conf.ProcessConf, p *Process, kubeNode *KubeNode, cluster *Cluster, args ...interface{}) error
type createProcessFunc func() *Process

//ProcessType kubenode中process类型常量
type ProcessType string

const (
	Process_MASTERWRAP       ProcessType = "masterwrap"
	Process_ETCD             ProcessType = "etcd"
	Process_ETCDHC           ProcessType = "etcdhc"
	Process_APISERVER        ProcessType = "apiserver"
	Process_CONTROLLER       ProcessType = "controller"
	Process_SCHEDULER        ProcessType = "scheduler"
	Process_MASTERDOCKER     ProcessType = "masterdocker"
	Process_INITMASTERCALICO ProcessType = "initmastercalico"
	Process_MASTERHC         ProcessType = "masterhc"
	Process_CRBUILDING       ProcessType = "crbuilding"
	Process_NODEWRAP         ProcessType = "nodewrap"
	Process_NODECONF         ProcessType = "nodeconf"
	Process_NODEDOCKER       ProcessType = "nodedocker"
	Process_KUBELET          ProcessType = "kubelet"
	Process_INITNODECALICO   ProcessType = "initnodecalico"
	Process_KUBELETHC        ProcessType = "kubelethc"
	Process_KUBEPROXY        ProcessType = "kubeproxy"
	Process_REMOVENODE       ProcessType = "removenode"
	Process_CONSOLEWRAP      ProcessType = "consolewrap"
	Process_KUBECTLCONF      ProcessType = "kubeconf"
	Process_GOTTY            ProcessType = "gotty"
	Process_CKEPROXY         ProcessType = "ckeproxy"
)

func (p ProcessType) String() string {
	switch p {
	case Process_MASTERWRAP:
		return "masterwrap"
	case Process_ETCD:
		return "etcd"
	case Process_ETCDHC:
		return "etcdhc"
	case Process_APISERVER:
		return "apiserver"
	case Process_CONTROLLER:
		return "controller"
	case Process_SCHEDULER:
		return "scheduler"
	case Process_MASTERDOCKER:
		return "masterdocker"
	case Process_INITMASTERCALICO:
		return "initmastercalico"
	case Process_MASTERHC:
		return "masterhc"
	case Process_CRBUILDING:
		return "crbuilding"
	case Process_NODEWRAP:
		return "nodewrap"
	case Process_NODECONF:
		return "nodeconf"
	case Process_NODEDOCKER:
		return "nodedocker"
	case Process_KUBELET:
		return "kubelet"
	case Process_INITNODECALICO:
		return "initnodecalico"
	case Process_KUBELETHC:
		return "kubelethc"
	case Process_KUBEPROXY:
		return "kubeproxy"
	case Process_REMOVENODE:
		return "removenode"
	case Process_CONSOLEWRAP:
		return "consolewrap"
	case Process_KUBECTLCONF:
		return "kubeconf"
	case Process_GOTTY:
		return "gotty"
	case Process_CKEPROXY:
		return "ckeproxy"
	default:
		return "other"
	}
}

func (p ProcessType) getCustomizeTaskFunc() CustomizeTask {
	switch p {
	case Process_ETCD:
		return CustomizeEtcd
	case Process_ETCDHC:
		return CustomizeEtcdHc
	case Process_APISERVER:
		return CustomizeAPIServer
	case Process_CONTROLLER:
		return CustomizeController
	case Process_SCHEDULER:
		return CustomizeScheduler
	case Process_MASTERDOCKER:
		return CustomizeDocker
	case Process_INITMASTERCALICO:
		return CustomizeCalico
	case Process_MASTERHC:
		return CustomizeMasterHc
	case Process_CRBUILDING:
		return CustomizeCrBuilding
	case Process_NODECONF:
		return CustomizeNodeConf
	case Process_KUBELET:
		return CustomizeKubelet
	case Process_NODEDOCKER:
		return CustomizeDocker
	case Process_KUBEPROXY:
		return CustomizeKubeProxy
	case Process_INITNODECALICO:
		return CustomizeCalico
	case Process_KUBELETHC:
		return CustomizeKubeletHc
	case Process_REMOVENODE:
		return CustomizeRemoveNode
	case Process_KUBECTLCONF:
		return CustomizeKubectlConf
	case Process_GOTTY:
		return CustomizeGotty
	case Process_CKEPROXY:
		return CustomizeCkeProxy
	}
	return nil
}

func (p ProcessType) CreateProcess() *Process {
	_createProcess := func(t ProcessType, cpus float64, mem float64) *Process {
		return &Process{
			Name: p.String(),
			Type: t,
			Res: &cluster.Resource{
				CPU: cpus,
				Mem: mem,
			},
		}
	}
	switch p {
	case Process_ETCD:
		return nil
	case Process_ETCDHC:
		return _createProcess(p, 0.1, 10)
	case Process_APISERVER:
		return nil
	case Process_CONTROLLER:
		return nil
	case Process_SCHEDULER:
		return nil
	case Process_MASTERDOCKER:
		return nil
	case Process_INITMASTERCALICO:
		return _createProcess(p, 0.1, 10)
	case Process_MASTERHC:
		return _createProcess(p, 0.1, 10)
	case Process_CRBUILDING:
		return _createProcess(p, 0.1, 10)
	case Process_NODECONF:
		return _createProcess(p, 0.1, 10)
	case Process_KUBELET:
		return nil
	case Process_NODEDOCKER:
		return nil
	case Process_KUBEPROXY:
		return nil
	case Process_INITNODECALICO:
		return _createProcess(p, 0.1, 10)
	case Process_KUBELETHC:
		return _createProcess(p, 0.1, 10)
	case Process_REMOVENODE:
		return _createProcess(p, 0.1, 10)
	case Process_KUBECTLCONF:
		return _createProcess(p, 0.1, 10)
	case Process_GOTTY:
		return nil
	case Process_CKEPROXY:
		return nil
	}
	return nil
}
func (p ProcessType) getTargetStatus() cluster.ExpectStatus {
	switch p {
	case Process_ETCDHC, Process_MASTERHC, Process_CRBUILDING, Process_NODECONF, Process_INITNODECALICO, Process_KUBELETHC, Process_REMOVENODE, Process_INITMASTERCALICO, Process_KUBECTLCONF: //, Process_INITCOREDNS:
		return cluster.ExpectFINISHED
	}
	return cluster.ExpectRUN
}
