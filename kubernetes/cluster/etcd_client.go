package cluster

import (
	"bytes"
	"cke/cluster"
	clr "cke/cluster"
	"cke/kubernetes"
	"cke/log"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/pkg/types"
	"github.com/golang/protobuf/proto"
	"gopkg.in/yaml.v2"
	"cke/kubernetes/cluster/conf"
)

/**
  k8s集群后端存储 etcd实现
*/
type EtcdClient struct {
	etcdCli       *clientv3.Client //clientv3客户端
	isRunning     bool
	frameworkName *string
	clusterName   *string
}

func NewEtcdClient(clusterName *string) Storage {
	if len(clr.StorageServers) == 0 {
		return &EtcdClient{
			isRunning:     false,
			clusterName:   clusterName,
			frameworkName: &clr.FrameworkName,
		}
	}

	etcds := strings.Split(clr.StorageServers, ",")
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   etcds, //[]string{"localhost:2379", "localhost:22379", "localhost:32379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil
	}
	return &EtcdClient{
		etcdCli:       cli,
		isRunning:     true,
		clusterName:   clusterName,
		frameworkName: &cluster.FrameworkName,
	}

}

func (ec *EtcdClient) Close() error {
	if ec.etcdCli == nil {
		return nil
	}
	ec.isRunning = false

	if ec.etcdCli != nil {
		if err := ec.etcdCli.Close(); err != nil {
			return err
		}
		ec.etcdCli = nil
	}
	return nil
}

func (ec *EtcdClient) goPutEtcdExec(ctx context.Context, wg *sync.WaitGroup, key, val string, opts ...clientv3.OpOption) {
	wg.Add(1)
	go func(wg *sync.WaitGroup, ctx context.Context, key, val string, opts ...clientv3.OpOption) {
		for {
			//contx, _ := context.WithTimeout(context.Background(), time.Second)
			//	startTime := time.Now()
			_, err := ec.etcdCli.Put(ctx, key, val, opts...)
			//	fmt.Printf("time: %+v,>>>>>>>>,key %+v,>>>>>>>>>key.size:%+v,>>>>>>>>>value.size:%+v", time.Since(startTime),key, len(key), len(val))
			if err != nil {
				log.Errorf("put data to storage error, key [%+v] value [%+v], error: %+v", key, val, err.Error())
			} else {
				break
			}
			time.Sleep(2 * time.Second)
		}
		wg.Done()
	}(wg, ctx, key, val, opts...)
}

func (ec *EtcdClient) getCkePath() string {
	return "/CKE/" + *ec.frameworkName
}

func (ec *EtcdClient) getLeaderKey() string {
	return ec.getCkePath() + "/leader"
}

func (ec *EtcdClient) getFrameworkIdKey() string {
	return ec.getCkePath() + "/frameowrk/id"
}

func (ec *EtcdClient) getClusterPath() string {
	return ec.getCkePath() + "/clusters/" + reflect.TypeOf(Cluster{}).String() + ":" + *ec.clusterName
}

func (ec *EtcdClient) getNodesPath() string {
	return ec.getClusterPath() + "/kubenodes"
}

func (ec *EtcdClient) getNodePath(node *KubeNode) string {
	return ec.getNodesPath() + "/" + node.Name
}

func (ec *EtcdClient) getProcessesPath(node *KubeNode) string {
	return ec.getNodePath(node) + "/Processes"
}

func (ec *EtcdClient) getProcessPath(node *KubeNode, process *Process) string {
	return ec.getProcessesPath(node) + "/" + process.Name
}

//持久化
func (ec *EtcdClient) StoreCluster(cluster *Cluster) error {
	if ec.etcdCli == nil {
		return errors.New("Etcd client is nil.")
	}
	startTime := time.Now()
	wg := new(sync.WaitGroup)
	ctx := context.Background()

	if cluster != nil {
		ec.goPutEtcdExec(ctx, wg, ec.getClusterPath()+"/ClusterName", cluster.ClusterName)
		ec.goPutEtcdExec(ctx, wg, ec.getClusterPath()+"/Expect", strconv.Itoa(int(cluster.Expect)))
		ec.goPutEtcdExec(ctx, wg, ec.getClusterPath()+"/Network", cluster.Network)

		crts, err := json.Marshal(cluster.certs)
		if err != nil {
			return err
		}
		ec.goPutEtcdExec(ctx, wg, ec.getClusterPath()+"/certs", string(crts))
		tks, err := json.Marshal(cluster.tokens)
		if err != nil {
			return err
		}
		ec.goPutEtcdExec(ctx, wg, ec.getClusterPath()+"/tokens", string(tks))
		//ingresses, err := json.Marshal(cluster.Ingresses)
		ingress, err := json.Marshal(cluster.Ingress)
		if err != nil {
			return err
		}
		//ec.goPutEtcdExec(ctx, wg, ec.getClusterPath()+"/Ingresses", string(ingresses))
		ec.goPutEtcdExec(ctx, wg, ec.getClusterPath()+"/Ingress", string(ingress))

		for _, kn := range cluster.KubeNodes {
			ec._storeKubeNode(ctx, wg, kn)
		}

		ec._storeSharedata(ctx, wg, cluster.shareData)

		ec._storeConf(ctx, wg, cluster.Conf)
	}
	wg.Wait()
	log.Infof("store cluster duration: %v", time.Since(startTime))
	return nil
}

//恢复内存
func (ec *EtcdClient) FetchCluster(cluster *Cluster) error {
	if ec.etcdCli == nil {
		return errors.New("Etcd client is nil.")
	}
	if cluster != nil {
		rsp, err := ec.etcdCli.Get(context.Background(), ec.getClusterPath()+"/ClusterName", clientv3.WithPrefix())
		if err != nil {
			return err
		}
		for _, kv := range rsp.Kvs {
			cluster.ClusterName = string(kv.Value)
		}

		rsp, err = ec.etcdCli.Get(context.Background(), ec.getClusterPath()+"/Expect", clientv3.WithPrefix())
		if err != nil {
			return err
		}
		for _, kv := range rsp.Kvs {
			if state, err := strconv.Atoi(string(kv.Value)); err == nil {
				cluster.Expect = clr.ExpectStatus(state)
			} else {
				return err
			}
		}

		rsp, err = ec.etcdCli.Get(context.Background(), ec.getClusterPath()+"/Network", clientv3.WithPrefix())
		if err != nil {
			return err
		}
		for _, kv := range rsp.Kvs {
			cluster.Network = string(kv.Value)
		}

		rsp, err = ec.etcdCli.Get(context.Background(), ec.getClusterPath()+"/certs", clientv3.WithPrefix())
		if err != nil {
			return err
		}
		for _, kv := range rsp.Kvs {
			json.Unmarshal(kv.Value, &cluster.certs)
		}

		rsp, err = ec.etcdCli.Get(context.Background(), ec.getClusterPath()+"/tokens", clientv3.WithPrefix())
		if err != nil {
			return err
		}
		for _, kv := range rsp.Kvs {
			json.Unmarshal(kv.Value, &cluster.tokens)
		}

		//rsp, err = ec.etcdCli.Get(context.Background(), ec.getClusterPath()+"/Ingresses", clientv3.WithPrefix())
		rsp, err = ec.etcdCli.Get(context.Background(), ec.getClusterPath()+"/Ingress", clientv3.WithPrefix())
		if err != nil {
			return err
		}
		for _, kv := range rsp.Kvs {
			json.Unmarshal(kv.Value, &cluster.Ingress)
		}

		ec.FetchConf(cluster)

		rsp, err = ec.etcdCli.Get(context.Background(), ec.getNodesPath(), clientv3.WithPrefix(), clientv3.WithKeysOnly())
		if err != nil {
			return err
		}
		keys := types.NewUnsafeSet()
		for _, kv := range rsp.Kvs {
			keySplits := strings.Split(strings.TrimPrefix(string(kv.Key), ec.getNodesPath()), "/")
			keys.Add(keySplits[1])
		}
		for _, nodeName := range keys.Values() {
			node := &KubeNode{
				Name: nodeName,
				cluster: cluster,
			}
			ec.FetchKubenode(node)
			cluster.AddNodeToSlice(node)
		}

		ec.FetchSharedata(cluster.shareData, cluster.KubeNodes)

	}
	return nil
}

func (ec *EtcdClient) DeleteCluster(cluster *Cluster) error {
	if ec.etcdCli == nil {
		return errors.New("Etcd client is nil.")
	}
	if cluster != nil {
		if _, err := ec.etcdCli.Delete(context.Background(), ec.getClusterPath(), clientv3.WithPrefix()); err != nil {
			return err
		}
	}

	return nil
}

//StoreSharedata add and update share data of cluster
func (ec *EtcdClient) StoreSharedata(shareData *ShareData) error {
	wg := new(sync.WaitGroup)
	err := ec._storeSharedata(context.Background(), wg, shareData)
	wg.Wait()
	return err
}

func (ec *EtcdClient) _storeSharedata(ctx context.Context, wg *sync.WaitGroup, shareData *ShareData) error {
	masterIps, err := json.Marshal(shareData.masterIPs.Values())
	if err != nil {
		log.Errorf("json Marshal error: %+v", err.Error())
	}
	ec.goPutEtcdExec(ctx, wg, ec.getClusterPath()+"/sharedata/masterips", string(masterIps))

	nodeIps, err := json.Marshal(shareData.nodeIps.Values())
	if err != nil {
		log.Errorf("json Marshal error: %+v", err.Error())
	}
	ec.goPutEtcdExec(ctx, wg, ec.getClusterPath()+"/sharedata/nodeips", string(nodeIps))

	ec.goPutEtcdExec(ctx, wg, ec.getClusterPath()+"/sharedata/etcdserver", shareData.etcdServer.String())

	return nil
}

func (ec *EtcdClient) FetchSharedata(shareData *ShareData, nodes []*KubeNode) error {

	if ec.etcdCli == nil {
		return nil
	}
	rsp, err := ec.etcdCli.Get(context.Background(), ec.getClusterPath()+"/sharedata/masterips", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	var masterIpsValue []string
	for _, kv := range rsp.Kvs {
		json.Unmarshal(kv.Value, &masterIpsValue)
	}
	shareData.masterIPs = types.NewUnsafeSet(masterIpsValue...)

	rsp, err = ec.etcdCli.Get(context.Background(), ec.getClusterPath()+"/sharedata/nodeips", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	var nodeIpsValue []string
	for _, kv := range rsp.Kvs {
		json.Unmarshal(kv.Value, &nodeIpsValue)
	}
	shareData.nodeIps = types.NewUnsafeSet(nodeIpsValue...)

	rsp, err = ec.etcdCli.Get(context.Background(), ec.getClusterPath()+"/sharedata/etcdserver", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kv := range rsp.Kvs {
		shareData.etcdServer = bytes.NewBuffer(kv.Value)
	}

	for _, node := range nodes {
		for _, process := range node.Processes {
			if process.Type == Process_MASTERHC {
				shareData.masterWithHc = node
				break
			}
		}
		if shareData.masterWithHc != nil {
			break
		}
	}

	return nil
}

func (ec *EtcdClient) StoreConf(cluster *Cluster) error {
	wg := new(sync.WaitGroup)
	err := ec._storeConf(context.Background(), wg, cluster.Conf)
	wg.Wait()
	return err
}

func (ec *EtcdClient) _storeConf(ctx context.Context, wg *sync.WaitGroup, ckeConf *conf.CKEConf) error {
	var confPath string
	confPath = ec.getClusterPath() + "/Conf"

	confBytes, err := yaml.Marshal(ckeConf)
	if err != nil {
		log.Errorf("yaml Marshal error: %+v", err.Error())
	}
	ec.goPutEtcdExec(ctx, wg, confPath, string(confBytes))
	return nil
}

func (ec *EtcdClient) FetchConf(cluster *Cluster) error {
	var confPath string
	confPath = ec.getClusterPath() + "/Conf"

	rs, err := ec.etcdCli.Get(context.Background(), confPath, clientv3.WithPrefix())
	if err != nil {
		return err
	}
	cluster.Conf =  &conf.CKEConf{}
	for _, kv := range rs.Kvs {
		yaml.Unmarshal(kv.Value, &cluster.Conf)
	}
	//设置端口
	cluster.gottyPort = *cluster.Conf.GetVariable("GOTTY_PORT")
	cluster.apiServerProxyPort = *cluster.Conf.GetVariable("API_PROXY_PORT")
	cluster.dashBoardProxyPort = *cluster.Conf.GetVariable("DASH_PROXY_PORT")
	cluster.gatewayManagePort = *cluster.Conf.GetVariable("MANAGE_PORT")

	return nil
}

//StoreKubeNode add  and update kubenode
func (ec *EtcdClient) StoreKubeNode(node *KubeNode) error {
	if ec.etcdCli == nil {
		return errors.New("Etcd client is nil.")
	}
	wg := new(sync.WaitGroup)
	err := ec._storeKubeNode(context.Background(), wg, node)
	wg.Wait()
	return err
}
func (ec *EtcdClient) _storeKubeNode(ctx context.Context, wg *sync.WaitGroup, node *KubeNode) error {
	nodePath := ec.getNodePath(node)
	ec.goPutEtcdExec(ctx, wg, nodePath+"/Name", node.Name)
	ec.goPutEtcdExec(ctx, wg, nodePath+"/Type", string(node.Type))
	ec.goPutEtcdExec(ctx, wg, nodePath+"/Host", node.Host)
	if len(node.RealHost) > 0 {
		ec.goPutEtcdExec(ctx, wg, nodePath+"/RealHost", node.RealHost)
	}

	ec.goPutEtcdExec(ctx, wg, nodePath+"/NodeIP", node.NodeIP)

	ec.goPutEtcdExec(ctx, wg, nodePath+"/NodeID", node.NodeID)
	//ingresses, err := json.Marshal(node.Ingresses)
	ingress, err := json.Marshal(node.Ingress)
	if err != nil {
		log.Errorf("json Marshal error: %+v", err.Error())
	}
	//ec.goPutEtcdExec(ctx, wg, nodePath+"/Ingresses", string(ingresses))
	ec.goPutEtcdExec(ctx, wg, nodePath+"/Ingress", string(ingress))
	ec.goPutEtcdExec(ctx, wg, nodePath+"/Expect", strconv.Itoa(int(node.Expect)))

	for _, pro := range node.Processes {
		ec._storeProcess(ctx, wg, node, pro)
	}

	//存储reservation信息
	ec._storeRervation(ctx, wg, node, node.Reservation)

	ec._storeTask(ctx, wg, node, nil, node.Task)

	return nil
}

func (ec *EtcdClient) FetchKubenode(node *KubeNode) error {
	if ec.etcdCli == nil {
		return nil
	}
	nodePath := ec.getNodePath(node)
	rs, err := ec.etcdCli.Get(context.Background(), nodePath+"/Name", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kv := range rs.Kvs {
		node.Name = string(kv.Value)
	}
	rs, err = ec.etcdCli.Get(context.Background(), nodePath+"/Type", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kv := range rs.Kvs {
		node.Type = KubeNodeType(kv.Value)
	}

	rs, err = ec.etcdCli.Get(context.Background(), nodePath+"/Host", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kv := range rs.Kvs {
		node.Host = string(kv.Value)
	}

	rs, err = ec.etcdCli.Get(context.Background(), nodePath+"/RealHost", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kv := range rs.Kvs {
		node.RealHost = string(kv.Value)
	}

	rs, err = ec.etcdCli.Get(context.Background(), nodePath+"/NodeIP", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kv := range rs.Kvs {
		node.NodeIP = string(kv.Value)
	}

	rs, err = ec.etcdCli.Get(context.Background(), nodePath+"/NodeID", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kv := range rs.Kvs {
		node.NodeID = string(kv.Value)
	}

	//rs, err = ec.etcdCli.Get(context.Background(), nodePath+"/Ingresses", clientv3.WithPrefix())
	rs, err = ec.etcdCli.Get(context.Background(), nodePath+"/Ingress", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kv := range rs.Kvs {
		//json.Unmarshal(kv.Value, &node.Ingresses)
		json.Unmarshal(kv.Value, &node.Ingress)
	}

	rs, err = ec.etcdCli.Get(context.Background(), nodePath+"/Expect", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kv := range rs.Kvs {
		if state, err := strconv.Atoi(string(kv.Value)); err == nil {
			node.Expect = clr.ExpectStatus(state)
		} else {
			return err
		}
	}
    //设置node conf
    node.Conf = node.cluster.Conf.GetNodeConf(string(node.Type))

	//获取预留信息
	ec.FetchReservation(node)

	rs, err = ec.etcdCli.Get(context.Background(), nodePath+"/Processes", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	keys := types.NewUnsafeSet()
	for _, kv := range rs.Kvs {
		keySplits := strings.Split(strings.TrimPrefix(string(kv.Key), nodePath+"/Processes"), "/")
		keys.Add(keySplits[1])
	}
	node.Processes = make([]*Process, 0)
	for _, proName := range keys.Values() {
		process := &Process{
			Name: proName,
		}
		err = ec.FetchProcess(node, process)
		if err != nil {
			return err
		}
		node.Processes = append(node.Processes, process)
	}

	node.Task = &cluster.Task{}
	if err = ec.FetchTask(node, nil, node.Task); err != nil {
		return err
	}

	return nil
}

func (ec *EtcdClient) DeleteKubenode(node *KubeNode) error {
	if ec.etcdCli == nil {
		return errors.New("Etcd client is nil.")
	}

	if _, err := ec.etcdCli.Delete(context.Background(), ec.getNodePath(node), clientv3.WithPrefix()); err != nil {
		log.Error("Deleted node error: ", err.Error())
		return err
	}
	log.Debug("Deleted node " + node.Name + "from storage.")
	return nil
}

func (ec *EtcdClient) StoreProcess(node *KubeNode, process *Process) error {
	wg := new(sync.WaitGroup)
	err := ec._storeProcess(context.Background(), wg, node, process)
	wg.Wait()
	return err
}

func (ec *EtcdClient) _storeProcess(ctx context.Context, wg *sync.WaitGroup, node *KubeNode, process *Process) error {
	processPath := ec.getProcessPath(node, process)
	ec.goPutEtcdExec(ctx, wg, processPath+"/Name", process.Name)
	ec.goPutEtcdExec(ctx, wg, processPath+"/Type", string(process.Type))

	//保存Resource信息
	pres, err := json.Marshal(process.Res)
	if err != nil {
		log.Errorf("json Marshal error: %+v", err.Error())
	}
	ec.goPutEtcdExec(ctx, wg, processPath+"/Res", string(pres))

	poption, err := json.Marshal(process.Options)
	if err != nil {
		log.Errorf("json Marshal error: %+v", err.Error())
	}
	ec.goPutEtcdExec(ctx, wg, processPath+"/Options", string(poption))

	ec.goPutEtcdExec(ctx, wg, processPath+"/Cmd", process.Cmd)

	pconfigjson, err := json.Marshal(process.ConfigJSON)
	if err != nil {
		log.Errorf("json Marshal error: %+v", err.Error())
	}
	ec.goPutEtcdExec(ctx, wg, processPath+"/ConfigJson", string(pconfigjson))

	ec.goPutEtcdExec(ctx, wg, processPath+"/Expect", strconv.Itoa(process.Expect.Int()) )

	ec._storeTask(ctx, wg, node, process, process.Task)

	return nil
}

func (ec *EtcdClient) FetchProcess(node *KubeNode, process *Process) error {
	if ec.etcdCli == nil {
		return errors.New("Etcd client is nil.")
	}
	processPath := ec.getProcessPath(node, process)
	rs, err := ec.etcdCli.Get(context.Background(), processPath+"/Name", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kv := range rs.Kvs {
		process.Name = string(kv.Value)
	}

	rs, err = ec.etcdCli.Get(context.Background(), processPath+"/Type", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kv := range rs.Kvs {
		process.Type = ProcessType(kv.Value)
	}

	rs, err = ec.etcdCli.Get(context.Background(), processPath+"/Res", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kv := range rs.Kvs {
		json.Unmarshal(kv.Value, &process.Res)
	}

	rs, err = ec.etcdCli.Get(context.Background(), processPath+"/Options", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kv := range rs.Kvs {
		process.Options = []string{}
		json.Unmarshal(kv.Value, &process.Options)
	}

	rs, err = ec.etcdCli.Get(context.Background(), processPath+"/Cmd", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kv := range rs.Kvs {
		process.Cmd = string(kv.Value)
	}

	rs, err = ec.etcdCli.Get(context.Background(), processPath+"/ConfigJson", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kv := range rs.Kvs {
		json.Unmarshal(kv.Value, &process.ConfigJSON)
	}

	rs, err = ec.etcdCli.Get(context.Background(), processPath+"/Expect", clientv3.WithPrefix())
	if err != nil {
		return err
	}

	for _, kv := range rs.Kvs {
		expect, err := strconv.Atoi(string(kv.Value))
		if err != nil {
			return err
		}
		process.Expect = clr.ExpectStatus(expect)
	}

	process.Task = &cluster.Task{}
	ec.FetchTask(node, process, process.Task)

	return nil
}

func (ec *EtcdClient) StoreTask(node *KubeNode, process *Process, task *cluster.Task) error {
	wg := new(sync.WaitGroup)
	err := ec._storeTask(context.Background(), wg, node, process, task)
	wg.Wait()
	return err
}

func (ec *EtcdClient) _storeTask(ctx context.Context, wg *sync.WaitGroup, node *KubeNode, process *Process, task *cluster.Task) error {
	//store task Info and Status 存储任务的status
	//针对kube node中的wrapper task
	var taskPath string
	if process == nil {
		taskPath = ec.getNodePath(node) + "/Task"
	} else {
		taskPath = ec.getProcessPath(node, process) + "/Task"
	}

	taskInfoPath := taskPath + "/Info"
	taskStatusPath := taskPath + "/Status"

	//info
	ec.goPutEtcdExec(context.Background(), wg, taskInfoPath+"/Name", task.Info.Name)
	ec.goPutEtcdExec(context.Background(), wg, taskInfoPath+"/Type", task.Info.Type)
	if task.Info.Host != nil {
		ec.goPutEtcdExec(context.Background(), wg, taskInfoPath+"/Host", *task.Info.Host)
	}

	k8sTask, ok := task.Info.Body.(*kubernetes.K8STask)
	if !ok {
		log.Error("TaskInfo Body is not a K8STask")
	} else {
		taskBytes, err := proto.Marshal(k8sTask)
		if err != nil {
			log.Errorf("json Marshal error: %+v", err.Error())
		} else {
			ec.goPutEtcdExec(context.Background(), wg, taskInfoPath+"/Info", string(taskBytes))
		}
	}

	taskInfoRes, err := json.Marshal(task.Info.Res)
	if err != nil {
		log.Errorf("json Marshal error: %+v", err.Error())
	}
	ec.goPutEtcdExec(context.Background(), wg, taskInfoPath+"/Res", string(taskInfoRes))

	taskInfoExecutor, err := json.Marshal(task.Info.Executor)
	if err != nil {
		log.Errorf("json Marshal error: %+v", err.Error())
	}
	ec.goPutEtcdExec(context.Background(), wg, taskInfoPath+"/Executor", string(taskInfoExecutor))

	taskInfoState, err := json.Marshal(task.Info.State)
	if err != nil {
		log.Errorf("json Marshal error: %+v", err.Error())
	}
	ec.goPutEtcdExec(context.Background(), wg, taskInfoPath+"/State", string(taskInfoState))

	//status
	taskStatus, err := json.Marshal(task.Status)
	if err != nil {
		log.Errorf("json Marshal error: %+v", err.Error())
	}
	ec.goPutEtcdExec(context.Background(), wg, taskStatusPath, string(taskStatus))

	return nil
}

func (ec *EtcdClient) FetchTask(node *KubeNode, process *Process, task *cluster.Task) error {
	//fetch task Info and Status 读取任务的status
	var taskPath string
	if process == nil {
		taskPath = ec.getNodePath(node) + "/Task"
	} else {
		taskPath = ec.getProcessPath(node, process) + "/Task"
	}

	taskInfoPath := taskPath + "/Info"
	taskStatusPath := taskPath + "/Status"
	//info
	task.Info = &cluster.TaskInfo{
		Reservation: node.Reservation,
	}
	rs, err := ec.etcdCli.Get(context.Background(), taskInfoPath+"/Name", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kv := range rs.Kvs {
		task.Info.Name = string(kv.Value)
	}

	rs, err = ec.etcdCli.Get(context.Background(), taskInfoPath+"/Type", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kv := range rs.Kvs {
		task.Info.Type = string(kv.Value)
	}

	rs, err = ec.etcdCli.Get(context.Background(), taskInfoPath+"/Host", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kv := range rs.Kvs {
		str := string(kv.Value)
		task.Info.Host = &str
	}

	rs, err = ec.etcdCli.Get(context.Background(), taskInfoPath+"/Info", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kv := range rs.Kvs {
		k8sTask := &kubernetes.K8STask{}
		err := proto.Unmarshal(kv.Value, k8sTask)
		if err != nil {
			return err
		}
		task.Info.Body = k8sTask
	}

	rs, err = ec.etcdCli.Get(context.Background(), taskInfoPath+"/Res", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kv := range rs.Kvs {
		json.Unmarshal(kv.Value, &task.Info.Res)
	}

	rs, err = ec.etcdCli.Get(context.Background(), taskInfoPath+"/Executor", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kv := range rs.Kvs {
		json.Unmarshal(kv.Value, &task.Info.Executor)
	}

	rs, err = ec.etcdCli.Get(context.Background(), taskInfoPath+"/State", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kv := range rs.Kvs {
		json.Unmarshal(kv.Value, &task.Info.State)
	}

	//status
	task.Status = &cluster.TaskStatus{}
	rs, err = ec.etcdCli.Get(context.Background(), taskStatusPath, clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kv := range rs.Kvs {
		json.Unmarshal(kv.Value, &task.Status)
	}

	return nil
}

func (ec *EtcdClient) StoreReservation(node *KubeNode, reservation *clr.Reservation) error {
	wg := new(sync.WaitGroup)
	err := ec._storeRervation(context.Background(), wg, node, reservation)
	wg.Wait()
	return err
}

func (ec *EtcdClient) _storeRervation(ctx context.Context, wg *sync.WaitGroup, node *KubeNode, reservation *clr.Reservation) error {
	var reservationPath string
	reservationPath = ec.getNodePath(node) + "/Reservation"

	reservBytes, err := json.Marshal(reservation)
	if err != nil {
		log.Errorf("json Marshal error: %+v", err.Error())
	}
	ec.goPutEtcdExec(ctx, wg, reservationPath, string(reservBytes))
	for _, pro := range node.Processes {
		ec._storeProcess(ctx, wg, node, pro)
	}
	return nil
}

func (ec *EtcdClient) FetchReservation(node *KubeNode) error {
	var reservationPath string
	reservationPath = ec.getNodePath(node) + "/Reservation"

	rs, err := ec.etcdCli.Get(context.Background(), reservationPath, clientv3.WithPrefix())
	if err != nil {
		return err
	}
	node.Reservation = &clr.Reservation{}
	for _, kv := range rs.Kvs {
		json.Unmarshal(kv.Value, &node.Reservation)
	}
	node.Reservation.GetUsedResource = node.getUsedResource
	return nil
}