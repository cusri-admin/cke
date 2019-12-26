package cluster

import (
	"bytes"
	cc "cke/cluster"
	"cke/kubernetes"
	"cke/kubernetes/cluster/conf"
	"cke/log"
	"cke/rsa"
	"cke/storage"
	"cke/utils"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"encoding/json"

	"cke/spider"

	"github.com/coreos/etcd/pkg/types"
	"github.com/golang/protobuf/proto"
)

//TODO: 命令行参数的校验，需要重构建立代码模式
var (
	lxcfsPath      = flag.String("k8s_lxcfs_path", "", "The path of LXCFS. if set this argument LXCFS will be used in KubeNode")
	execPath       = flag.String("k8s_executor", "cke-k8s-exec", "The process of mesos executor for CKE kubernetes")
	enableCFS      = flag.Bool("k8s_cfs", false, "Enable docker CFS in KubeNode")
	dockerPath     = flag.String("k8s_docker_path", "", "The work path of the docker in the cke container. If empty, <docker work path on the host>/cke will be used")
	dockerRegistry = flag.String("k8s_registry", "", "Docker registry address")
	registryCert   = flag.String("k8s_registry_cert", "/cke-scheduler/reg.mg.hcbss/reg.mg.hcbss.crt", "Docker registry cert for k8s cluster")
)

type ShareData struct {
	//masterIp地址
	masterIPs types.Set
	//kube node ip地址
	nodeIps types.Set
	//集群etcd servers
	etcdServer *bytes.Buffer
	//集群中含有masterHc的master
	masterWithHc *KubeNode
}

//Cluster 集群结构体
type Cluster struct {
	K8sVer             string          `json:"k8s_ver"` //集群将要启动的k8s版本
	ClusterName        string          `json:"name"`
	Expect             cc.ExpectStatus `json:"expect"`               //集群目标状态
	Network            string          `json:"network"`              //集群使用的contiv网络名称
	KubeNodes          []*KubeNode     `json:"nodes"`                //集群中各节点
	Ingress            *Ingress        `json:"ingress,omitempty"`    //集群的ingress描述
	Attributes         []*Attribute    `json:"attributes,omitempty"` //集群的扩展属性
	sch                cc.Scheduler    //通用cluster信息，主要包含任务数据及相关操作
	lock               sync.RWMutex    // 任务对象锁
	storageCli         Storage         //storage 客户端
	certs              map[string][]byte
	tokens             map[string]string
	shareData          *ShareData
	k8sFsm             *FSM          //k8s集群控制任务启动状态机
	publisher          *LogPublisher //已创建的日志流
	CreateTime         time.Time
	Conf               *conf.CKEConf
	originalMsg        string //集群原始报文
	apiServerProxyPort string //集群中console节点上为apiserver桥接出来的端口
	dashBoardProxyPort string //集群中console节点上为dashboard桥接出来的端口
	gottyPort          string //集群中console节点上为gotty桥接出来的端口
	gatewayManagePort  string //console节点上，配置svc cluster ip路由策略的管理端口
}

func init() {
	//默认向cluster manager中增加集群生成器
	cc.ClusterCreators[reflect.TypeOf(Cluster{}).String()] = func(clusterName string, sch cc.Scheduler) cc.ClusterInterface {
		return &Cluster{
			ClusterName: clusterName,
			sch:         sch,
		}
	}
}

//Initialize 初始化集群
func (c *Cluster) Initialize(sch cc.Scheduler) (err error) {
	log.Infof("Initialize kubernetes cluster %s ....", c.ClusterName)

	c.Conf, err = conf.LoadCKEConf(c.K8sVer)
	if err != nil {
		return err
	}
	//TODO: 待完善conf机制，使用flag自动更新conf信息

	//检测docker registry的配置
	//TODO: 配置信息待完善
	if len(*dockerRegistry) <= 0 {
		return errors.New("Docker registry need to be configured")
	}

	registry := getRegistryDNS()
	if registry != nil {
		c.Conf.PutVariable("DOCKER_REGISTRY", &registry.Host)
	}
	c.Conf.PutVariable("CLUSTER_NAME", &c.ClusterName)
	vport := strconv.Itoa(rand.Intn(20000) + 40000)
	c.Conf.PutVariable("DASHBOARD_VPORT", &vport)

	//向配置变量里添加ingress的类型
	if c.Ingress != nil {
		lbType := c.Ingress.Type.String()
		c.Conf.PutVariable("LB_TYPE", &lbType)
	}

	c.publisher = NewPublisher()
	c.certs = make(map[string][]byte)
	c.tokens = make(map[string]string)

	c.shareData = &ShareData{
		masterIPs:  types.NewUnsafeSet(),
		nodeIps:    types.NewUnsafeSet(),
		etcdServer: bytes.NewBuffer([]byte{}),
		//gottyPort:  "38080",
	}
	c.sch = sch
	c.CreateTime = time.Now()
	c.Expect = cc.ExpectRUN

	kv := strings.Split(c.Network, "-")
	if len(kv) >= 2 {
		c.Ingress.Vpc = kv[len(kv)-1]
	}

	//bridge don't need spiderLB endpoint
	if strings.Compare(c.Ingress.Type.String(), IngressType_Bridge.String()) == 0 {
		c.Ingress.SpiderLBEndpoint = ""
	} else {
		c.Ingress.SpiderLBEndpoint = *spider.LBEndpoint
	}

	if c.sch.GetShortFrameworkID() != "" && c.ClusterName != "" {
		c.Ingress.ServiceidPrefix = c.sch.GetShortFrameworkID() + "-" + c.ClusterName + "-"
	}

	//从contv中获取指定网络的ip pool
	if err = c.checkFormat(); err != nil {
		return err
	}
	//创建etcd客户端，用于同步数据
	c.storageCli = CreateStorage(c.ClusterName, storage.StorageType(strings.ToLower(cc.StorageType)))

	//初始化每个节点
	for _, node := range c.KubeNodes {
		if err := node.Initialize(c); err != nil {
			return err
		}
	}

	if err = c.generateK8sTask(); err != nil { //按照需求生成
		return err
	}

	if c.Ingress != nil && c.Ingress.Type == IngressType_SpiderLB {
		//将每个节点添加到spider
		var hosts []string
		for _, kubenode := range c.KubeNodes {
			hosts = append(hosts, kubenode.NodeIP)
		}
		if err = spider.AddHostsFromRR(c.Network, hosts); err != nil {
			return err
		}
	}

	c.initialFSM()
	go func() {
		c.k8sFsm.stopFsm()
		log.Infof("cluseter <%s> is storing all data to storage....", c.ClusterName)
		c.storageCli.StoreCluster(c) //存储集群信息到etcd中
		c.k8sFsm.startFsm()
	}()

	c.sch.ReviveOffer(c)
	return nil
}

//GetResource 返回这个集群应该使用的资源量
func (c *Cluster) GetResource() *cc.Resource {
	res := &cc.Resource{}
	for _, n := range c.KubeNodes {
		res.AddResource(n.GetResource())
	}
	//因为每个Mesos Task启动时都要判断Task Res + executor Res是否满足offer Res
	//所以如果按照Cluster正好的Res分配Role后，最后一个Task收到的Offer会因为缺少一个Executor Res，
	//导致以为资源不够而不启动，在此多加一个Executor Res。避免这种情况
	res.AddResource(getExecutorInfo(c.ClusterName, "").Res)
	res.AddResource(getExecutorInfo(c.ClusterName, "").Res)
	res.AddResource(&cc.Resource{
		CPU: 2,
		Mem: 1024,
	})
	return res
}

func (c *Cluster) recover(isNewFramework bool) {
	c.publisher = NewPublisher()
	c.KubeNodes = []*KubeNode{}
	c.certs = make(map[string][]byte)
	c.tokens = make(map[string]string)
	//c.Ingresses = &Ingresses{}
	c.shareData = &ShareData{}
	c.storageCli = CreateStorage(c.ClusterName, storage.StorageType(strings.ToLower(cc.StorageType)))
	c.storageCli.FetchCluster(c)
	c.initialFSM()

	if isNewFramework { //如果是新的fwid，并且storage中有集群数据，则需要根据新fw id重新生成集群数据，默认将所有旧任务删除
		c.trimOldInfo()
		if err := c.generateK8sTask(); err != nil { //按照需求生成
			log.Errorf("generate new cluster <%v> info error when recover cluster from storage, error: %v", c.ClusterName, err.Error())
		}
		go func() {
			c.k8sFsm.stopFsm()
			log.Infof("cluseter <%s> is storing all data to storage....", c.ClusterName)
			c.storageCli.StoreCluster(c) //存储集群信息到etcd中
			c.k8sFsm.startFsm()
		}()
	}
}

//建立集群层次结构状态机，cluster中分为两种状态机，一个是master（所有master状态机共享一个），一个是node状态机（每个node是独立状态机）
func (c *Cluster) initialFSM() {
	c.k8sFsm = NewFSM("master", &MasterWrapState, c.publisher, c, nil) //创建node类型状态机
	masters := make([]*KubeNode, 0)
	var oneMaster *KubeNode
	c.lock.RLock()
	for _, node := range c.KubeNodes {
		if node.Type == KubeNodeType_Master {
			node.fsm = c.k8sFsm //master类型节点共享一个状态机
			masters = append(masters, node)
			oneMaster = node
		} else if node.Type == KubeNodeType_Node {
			node.initialFSM(c, []*KubeNode{})
		} else {
			node.initialFSM(c, []*KubeNode{})
		}
	}
	c.lock.RUnlock()
	oneMaster.initialFSM(c, masters) //对master进行初始化
}

//删除从storage中获取到的老的集群任务，包括 finished 的任务及 长运行任务状态的状态
func (c *Cluster) trimOldInfo() {
	c.certs = make(map[string][]byte)
	c.tokens = make(map[string]string)
	c.shareData = &ShareData{
		masterIPs:  types.NewUnsafeSet(),
		nodeIps:    types.NewUnsafeSet(),
		etcdServer: bytes.NewBuffer([]byte{}),
	}
	c.lock.RLock()
	defer c.lock.RUnlock()
	for _, node := range c.KubeNodes {
		node.trimOldInfo()
	}
}

func getRegistryDNS() *kubernetes.K8SDnsRecord {
	if len(*dockerRegistry) > 0 {
		record := strings.Split(*dockerRegistry, ":")
		if len(record) == 2 {
			return &kubernetes.K8SDnsRecord{
				Host: record[0],
				Ip:   record[1],
			}
		}
	}
	return nil
}

//获得集群各节点的DNS记录
func (c *Cluster) getDNSRecord() []*kubernetes.K8SDnsRecord {
	records := make([]*kubernetes.K8SDnsRecord, 0)
	for _, ip := range c.shareData.masterIPs.Values() {
		dns := &kubernetes.K8SDnsRecord{
			Host: "kubernetes.default",
			Ip:   ip,
		}
		records = append(records, dns)
	}
	registry := getRegistryDNS()
	if registry != nil {
		records = append(records, registry)
	}
	return records
}

//从contiv中查找ip地址
func (c *Cluster) fetchIPPoolFromContiv(networkName string) []string {
	ipPool := []string{"10.124.220.20", "10.124.220.21", "10.124.220.22", "10.124.220.23", "10.124.220.24", "10.124.220.25"}
	return ipPool
}

//TODO: 应该转换成lambda func
func convert(i interface{}) interface{} {
	switch x := i.(type) {
	case map[interface{}]interface{}:
		m2 := map[string]interface{}{}
		for k, v := range x {
			m2[k.(string)] = convert(v)
		}
		return m2
	case []interface{}:
		for i, v := range x {
			x[i] = convert(v)
		}
	}
	return i
}

func getExecutorInfo(clusterName string, nodeName string) *cc.ExecutorInfo {
	args := make([]string, 0, 4)
	args = append(args, *execPath)
	args = append(args, "-log_level="+log.GetLevel().String())
	if *enableCFS {
		args = append(args, "-cfs")
	}
	if len(*lxcfsPath) > 0 {
		args = append(args, "-lxcfs="+(*lxcfsPath))
	}
	if len(*dockerPath) > 0 {
		args = append(args, "-docker_path="+(*dockerPath))
	}
	return &cc.ExecutorInfo{
		Name: fmt.Sprintf("%s.%s", clusterName, nodeName),
		Res: &cc.Resource{
			CPU: 0.05,
			Mem: 32,
		},
		Cmd:  *execPath,
		Args: args,
	}
}

//生成k8s集群的task
func (c *Cluster) generateK8sTask() error {
	//搜集kubenode中master节点的container ip地址
	ipPool := c.fetchIPPoolFromContiv(c.Network)
	//搜集kubenode中master节点的container ip地址
	ipadds := []string{"11.254.0.1", "127.0.0.1"}
	index := 0
	c.lock.RLock()
	for _, kubeNode := range c.KubeNodes {
		if len(kubeNode.NodeIP) > 0 { //如果配置了node的ip地址
			if kubeNode.Type == KubeNodeType_Master {
				ipadds = append(ipadds, kubeNode.NodeIP)
				c.shareData.masterIPs.Add(kubeNode.NodeIP)
			} else if kubeNode.Type == KubeNodeType_Node {
				c.shareData.nodeIps.Add(kubeNode.NodeIP)
			}

		} else { //如果没有配置node的ip地址
			if index >= len(ipPool) {
				c.lock.RUnlock()
				return errors.New("IP Pool of Contiv is not enough")
			}
			kubeNode.NodeIP = ipPool[index]
			if kubeNode.Type == KubeNodeType_Master {
				c.shareData.masterIPs.Add(ipPool[index])
				ipadds = append(ipadds, ipPool[index])
			} else if kubeNode.Type == KubeNodeType_Node {
				c.shareData.nodeIps.Add(kubeNode.NodeIP)
			}
			index++
		}
	}
	c.lock.RUnlock()
	//创建根证书
	rootCrt, rootKey, err := rsa.CreateRootCrtAndKey(false)
	if err != nil {
		return err
	}
	//创建自签名证书
	dnsNames := []string{"kubernetes",
		"kubernetes.default",
		"kubernetes.default.svc",
		"kubernetes.default.svc.cluster",
		"kubernetes.default.svc.cluster.local"}

	masterIPs := c.shareData.masterIPs.Values()
	sort.Strings(masterIPs)
	//masterIpArr[0]为最小ip地址，设置为证书common name
	serverCrt, serverKey, err := rsa.SignCrtAndKey(rootCrt, rootKey, c.ClusterName, masterIPs[0], ipadds, dnsNames, false)
	//存储到cluster中
	c.certs["rootKey"] = rootKey
	c.certs["rootCrt"] = rootCrt
	c.certs["serverKey"] = serverKey
	c.certs["serverCrt"] = serverCrt
	//生成kubelet-bootstrap token
	c.tokens["bootstrapToken"] = rsa.CreateToken()
	//生成admin用户的token
	c.tokens["adminToken"] = rsa.CreateToken()
	//配置docker registry cert
	var registryCertBytes []byte
	if strings.HasPrefix(*registryCert, "http://") {
		url := *registryCert
		rep, err := http.Get(url)
		if err != nil {
			return err
		}
		registryCertBytes, err = ioutil.ReadAll(rep.Body)
		if err != nil {
			return err
		}
	} else {
		registryCertBytes, err = ioutil.ReadFile(*registryCert)
		if err != nil {
			return err
		}
	}
	c.certs["registryCert"] = registryCertBytes

	//设置masterIpsExecutorInfo
	var corednsIPStr bytes.Buffer
	for idx, ip := range c.shareData.masterIPs.Values() {
		corednsIPStr.WriteString(ip)
		c.shareData.etcdServer.WriteString("https://")
		c.shareData.etcdServer.WriteString(ip)
		c.shareData.etcdServer.WriteString(":2389")
		if idx < c.shareData.masterIPs.Length()-1 {
			c.shareData.etcdServer.WriteString(",")
			corednsIPStr.WriteString(",")
		}
	}

	etcdServer := c.shareData.etcdServer.String()
	coreDNSIps := corednsIPStr.String()
	c.Conf.PutVariable("ETCD_SERVERS", &etcdServer)
	c.Conf.PutVariable("CORE_DNS_IPS", &coreDNSIps)

	//标记是否已经增加过master health check process，只需要在一个master上运行一次就行
	hasMasterHcPro := false
	//根据配置，将所有kubenode中所有process转化为cluster对应的k8s task对象
	c.lock.RLock()
	defer c.lock.RUnlock()
	for _, kubeNode := range c.KubeNodes {
		// 添加该KubeNode中的扩展process的任务
		for _, process := range kubeNode.Processes {
			//log.Println("k8s cluster generateK8sTask add task:" + taskInfo.Name)
			if process.Type == Process_ETCD && !hasMasterHcPro {
				//增加masterHc process
				masterHc := Process_MASTERHC.CreateProcess()
				kubeNode.Processes = append(kubeNode.Processes, masterHc)
				//记录该master地址，以便后期添加节点使用
				c.shareData.masterWithHc = kubeNode

				hasMasterHcPro = true
			}
		}
	}
	for _, kubeNode := range c.KubeNodes {
		log.Debugf("Add node: %s type: %s", kubeNode.Name, kubeNode.Type)
		// 添加启动KubeNode的任务
		err := kubeNode.generateK8sTask(c)
		if err != nil {
			return err
		}
	}
	return nil
}

//GetReservations 返回集群需要预留的资源信息
func (c *Cluster) GetReservations() []*cc.Reservation {
	rs := make([]*cc.Reservation, 0)
	for _, node := range c.KubeNodes {
		r := node.GetReservation()
		if r != nil {
			rs = append(rs, r)
		}
	}
	if len(rs) > 0 {
		return rs
	}
	return nil
}

//GetInconsistentTasks 获得正在变化中的tasks
func (c *Cluster) GetInconsistentTasks() (bool, []*cc.TaskInfo) {
	var tasks []*cc.TaskInfo
	if c.Expect == cc.ExpectRUN {
		_, tasks = c.k8sFsm.Call(NewFsmEvent(&ScheduleTaskType))
	}
	//集群在停止状态时，需要offer来确认删除预留
	return c.Expect == cc.ExpectRUN && c.k8sFsm.IsFinished(), tasks
}

//MarshalTask 将TaskInfo中的Body转换为protobuf 的[]byte以便scheduler发送给mesos的executor
func (c *Cluster) MarshalTask(taskBody interface{}) ([]byte, error) {
	k8sTask, ok := taskBody.(*kubernetes.K8STask)
	if ok {
		taskBytes, err := proto.Marshal(k8sTask)
		if err != nil {
			return nil, err
		}
		return taskBytes, nil
	}
	return nil, errors.New("TaskBody is not a K8STask")
}

//RemoveCluster 删除集群
func (c *Cluster) RemoveCluster() (*sync.WaitGroup, error) {
	var host []string
	c.Expect = cc.ExpectSTOP
	c.lock.RLock()
	defer c.lock.RUnlock()
	for _, kubeNodetmp := range c.KubeNodes {
		host = append(host, kubeNodetmp.NodeIP)
	}
	//TODO: 需等待集群删除完再执行收回？
	err := spider.DeleteLBServicesFromSpider(c.Network, c.sch.GetShortFrameworkID()+"-"+c.ClusterName)
	if err != nil {
		return nil, err
	}
	log.Infof("Begining to remove cluster %s", c.ClusterName)
	waitAllNodesDeleted := new(sync.WaitGroup)
	for _, kubeNode := range c.KubeNodes {
		c._deleteNode(kubeNode, waitAllNodesDeleted)
	}
	//删除集群其他信息
	go func() {
		waitAllNodesDeleted.Wait()
		c.storageCli.DeleteCluster(c)
	}()
	return waitAllNodesDeleted, nil
}

//GetName 获得集群名
func (c *Cluster) GetName() string {
	return c.ClusterName
}

//SetLeader 当前进程已经成为Leader
func (c *Cluster) SetLeader(isNewFramework bool) {
	//c.lock.Lock()
	//defer c.lock.Unlock()
	c.recover(isNewFramework)
}

//UpdateTaskStatus 更新集群中的任务状态，将被Manager调用
func (c *Cluster) UpdateTaskStatus(status *cc.TaskStatus) {
	//TODO: 需要与fsm.go中Call函数重构,用对象表示json
	var msg string
	if status.Message == nil {
		msg = fmt.Sprintf("Update task %s to %s", status.Id, status.State.String())
	} else {
		msg = fmt.Sprintf("Update task %s to %s msg: %s", status.Id, status.State.String(), *status.Message)
	}
	if status.State == cc.TASKMESOS_FAILED || status.State == cc.TASKMESOS_LOST {
		log.Error(msg)
		c.publisher.Log("{\"type\":\"log\",\"log\":\"[ERROR]" + strings.Replace(msg, "\"", "\\\"", -1) + "\"}")
	} else {
		log.Info(msg)
		c.publisher.Log("{\"type\":\"log\",\"log\":\"[INFO]" + strings.Replace(msg, "\"", "\\\"", -1) + "\"}")
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	for _, kubeNode := range c.KubeNodes {
		if kubeNode.UpdateTaskStatus(status, c) {
			break
		}
	}
	c.sch.ReviveOffer(c)
}

//AddKubeNodes 添加若干个节点
func (c *Cluster) AddKubeNodes(kubeNodes []*KubeNode) error {
	if c.Expect != cc.ExpectRUN {
		return errors.New("can't add a node to stopping cluster")
	}

	kubeNodeNames := types.NewUnsafeSet()
	c.lock.RLock()
	for _, node := range c.KubeNodes {
		kubeNodeNames.Add(node.Name)
	}
	c.lock.RUnlock()
	//检查名称是否重复
	for _, kntmp := range kubeNodes {
		if kubeNodeNames.Contains(kntmp.Name) {
			return errors.New("Node [" + kntmp.Name + "] has existed")
		}
	}

	//配置kubeNode 添加到集群中
	//从contv中获取指定网络的ip pool
	ipPool := c.fetchIPPoolFromContiv(c.Network)
	//为每个没有指定node ip的节点从IP pool中分配一个IP
	index := 0

	if c.Ingress != nil && c.Ingress.Type == IngressType_SpiderLB {
		var hosts []string
		indextmp := 0
		for _, kntmp := range kubeNodes {
			if kntmp.Type == KubeNodeType_Node {
				if len(kntmp.NodeIP) <= 0 {
					if indextmp >= len(ipPool) {
						return errors.New("IP Pool of Contiv is not enough")
					}
					kntmp.NodeIP = ipPool[indextmp]
					indextmp++
				}
				hosts = append(hosts, kntmp.NodeIP)
			}
		}
		if err := spider.AddHostsFromRR(c.Network, hosts); err != nil {
			return err
		}
	}

	for _, kn := range kubeNodes {
		if err := kn.Initialize(c); err != nil {
			return err
		}
		if kn.Type == KubeNodeType_Node {
			if len(kn.NodeIP) > 0 { //如果配置了node的ip地址
				c.shareData.nodeIps.Add(kn.NodeIP)
			} else { //如果没有配置node的ip地址
				if index >= len(ipPool) {
					return errors.New("IP Pool of Contiv is not enough")
				}
				kn.NodeIP = ipPool[index]
				c.shareData.nodeIps.Add(kn.NodeIP)
				index++
			}
			//初始化node的状态机并加入cluster的fsm中
			kn.initialFSM(c, append([]*KubeNode{}, c.shareData.masterWithHc))
			//加入cluster中
			c.AddNodeToSlice(kn)
			//生成kubeNode中的任务
			err := kn.generateK8sTask(c)
			if err != nil {
				return err
			}
			//将节点存储到storage
			c.storageCli.StoreKubeNode(kn)
			//更新存储cluster的conf
			c.storageCli.StoreConf(c)
			//更新
		} else {
			return errors.New("Manager node can't be added after the cluster has be created")
		}
	}

	//更新console节点中proxy访问k8s cluster ip的路由
	c.updateProxyRoute()

	//生成addNode事件，并发送给状态机
	c.k8sFsm.Call(NewFsmEvent(&AddNodeType))

	//更新Offer的接收
	c.sch.ReviveOffer(c)

	return nil
}

//DeleteKubeNodes 删除集群内的指定节点
//因为删除操作为异步操作，该方法返回sync.WaitGroup 锁用来同步。
//当sync.WaitGroup的wait返回后则表示所有节点已经完成删除操作(成功/出错)
func (c *Cluster) DeleteKubeNodes(nodeNames []string) (*sync.WaitGroup, error) {

	if c.Expect != cc.ExpectRUN {
		return nil, errors.New("Can't delete node to stopping cluster")
	}

	//deletedNodes := make(map[string]*KubeNode)
	for _, name := range nodeNames {
		node := c.GetKubeNodeByName(&name)
		if node == nil {
			return nil, errors.New("can not find kube node [ " + name + " ] in cluster [ " + c.ClusterName + " ]")
		} else if node.Type == KubeNodeType_Master {
			return nil, errors.New("can not delete kube master node [ " + name + " ] in cluster [ " + c.ClusterName + " ]")
		}
	}

	//删除节点需要打开Offer
	c.sch.ReviveOffer(c)

	waitAllNodesDeleted := new(sync.WaitGroup)
	for _, name := range nodeNames {
		kn := c.GetKubeNodeByName(&name)
		//先从k8s集群中清除节点
		removeChan := kn.clearNodeFromK8s(c)
		//删除节点时可能会返回等待用的removeChan，等待节点将自己从API Srver中删除
		//如果为空则表示进程已经是停止状态
		if removeChan != nil {
			log.Infof("Waiting for clear node %s from K8s", name)
			//创建协程，等待移除节点任务执行完成，从cluster.KubeNodes中删除
			go func(toDeleteNode *KubeNode, removeChan <-chan ListenType) {
				listenEvent := <-removeChan
				if listenEvent == Listen_Started {
					log.Infof("Cleared node %s from K8s", name)
					c._deleteNode(kn, waitAllNodesDeleted)
				}
			}(kn, removeChan)
		} else {
			c._deleteNode(kn, waitAllNodesDeleted)
		}
	}
	return waitAllNodesDeleted, nil
}

func (c *Cluster) _deleteNode(node *KubeNode, waitAllNodesDeleted *sync.WaitGroup) {
	if c.Ingress != nil && c.Ingress.Type == IngressType_SpiderLB {
		if err := spider.DeleteHostsFromRR(c.Network, []string{node.NodeIP}); err != nil {
			log.Warningf("Remove vpc port error: %s", err.Error())
		}
	}

	waitAllNodesDeleted.Add(1)
	log.Infof("Stopping node %s.%s", c.ClusterName, node.Name)
	node.StopKubeNode(c)
	go func() {
		node.ReleaseResource()
		//从集群中删除
		c.DeleteNodeFromSlice(node.Name)
		//删除shareData中数据
		c.shareData.nodeIps.Remove(node.NodeIP)
		//从cluster 状态机终移除
		c.k8sFsm.deleteChild(node.fsm)
		//从storage中移除节点
		c.storageCli.DeleteKubenode(node)
		//更新console节点中proxy访问k8s cluster ip的路由
		c.updateProxyRoute()
		//完成了一个节点的删除
		waitAllNodesDeleted.Done()
		log.Infof("Stopped node %s.%s", c.ClusterName, node.Name)
	}()
	return
}

//UpdateProcess 更新集群内一个进程的配置
func (c *Cluster) UpdateProcess(kubeNodes []*KubeNode) error {
	nodes := make(map[string]*KubeNode) //集群已存在节点
	c.lock.RLock()
	for _, kn := range c.KubeNodes {
		nodes[kn.Name] = kn
	}
	c.lock.RUnlock()

	for _, kubeNode := range kubeNodes {
		if nodes[kubeNode.Name] == nil {
			return errors.New("can not find kube node [ " + kubeNode.Name + " ] in cluster [ " + c.ClusterName + " ]")
		}
	}
	//停止相应的task
	errStr := ""
	for _, kn := range kubeNodes {
		if err := nodes[kn.Name].UpdateProcess(c, kn.Processes); err != nil {
			if len(errStr) > 0 {
				errStr += " & " + err.Error()
			} else {
				errStr += err.Error()
			}
		}
	}
	if len(errStr) > 0 {
		return errors.New(errStr)
	}
	return nil
}

//GetAdminToken 获得admin token
func (c *Cluster) GetAdminToken() string {
	return c.tokens["adminToken"]
}

//CreateLogsReader 创建读取日志的对象
func (c *Cluster) CreateLogsReader() *LogReader {
	return c.publisher.CreateReader()
}

//GetProcessStdFile 得到指定进程的日志文件
func (c *Cluster) GetProcessStdFile(nodeID string, processName string,
	fileName string, offset uint64, length uint64) ([]byte, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	for _, kn := range c.KubeNodes {
		if kn.Name == nodeID {
			return kn.getProcessStdFile(processName, fileName, offset, length)
		}
	}
	return nil, utils.NewHttpError(http.StatusNotFound, "KubeNode "+nodeID+" could not be found in "+c.GetName())
}

//CheckFormat 检查集群数据(json格式)的正确性
func (c *Cluster) checkFormat() error {
	//TODO: 校验cluster的各数据的业务正确性
	if len(c.ClusterName) <= 0 {
		return errors.New("Cluster name is null")
	}
	if len(c.KubeNodes) <= 0 {
		return errors.New("Cluster " + c.ClusterName + " has no nodes")
	}

	if c.Ingress != nil {
		//if err := c.Ingresses.CheckFormat(); err != nil {
		if err := c.Ingress.CheckFormat(); err != nil {
			return errors.New("Cluster \"" + c.ClusterName + "\" ingress error: " + err.Error())
		}
	}

	//判断各node是否重名
	nodes := make(map[string]*KubeNode)
	c.lock.RLock()
	defer c.lock.RUnlock()
	for _, node := range c.KubeNodes {
		if nodes[node.Name] != nil {
			return errors.New("KubeNode \"" + node.Name + "\" repeat definition")
		}
		nodes[node.Name] = node
	}
	return nil
}

//GetKubeNodeByName 根据kube node name获取kubenode
func (c *Cluster) GetKubeNodeByName(kubeNodeName *string) *KubeNode {
	c.lock.RLock()
	defer c.lock.RUnlock()
	for _, kn := range c.KubeNodes {
		if kn.Name == *kubeNodeName {
			return kn
		}
	}
	return nil
}

//DeleteNodeFromSlice 根据kube node name从[]kubenode中删除node
func (c *Cluster) DeleteNodeFromSlice(kubeNodeName string) *KubeNode {
	c.lock.Lock()
	defer c.lock.Unlock()
	for idx := 0; idx < len(c.KubeNodes); idx++ {
		node := c.KubeNodes[idx]
		if node.Name == kubeNodeName {
			c.KubeNodes = append(c.KubeNodes[:idx], c.KubeNodes[idx+1:]...)
			return node
		}
	}
	return nil
}

//AddNodeToSlice 根据kube node name向[]kubenode中添加node
func (c *Cluster) AddNodeToSlice(kubeNode *KubeNode) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.KubeNodes = append(c.KubeNodes, kubeNode)
	return
}

func (c *Cluster) GetGottyIp() (string, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	for _, kn := range c.KubeNodes {
		if kn.Type == KubeNodeType_Console {
			gottyPort := *kn.Conf.GetVariable("GOTTY_PORT")
			if kn.RealHost != "" && kn.RealHost != "*" && len(gottyPort) > 0 {
				return kn.RealHost + ":" + gottyPort, nil
			}
		}
	}

	return "", errors.New("can not find gotty server in cluster [ " + c.ClusterName + " ]")
}

func (c *Cluster) GetDashboardIpPortFromSpiderLB() (string, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	lbVip, err := spider.GetVIP(c.Network)
	if err != nil {
		log.Errorf("Fetch vip form Spider error: %s", err.Error())
		return "", fmt.Errorf("can not find dashboard server in cluster: %s, err: %s", c.ClusterName, err.Error())
	}
	return lbVip + ":" + *c.Conf.GetVariable("DASHBOARD_VPORT"), nil
}

func (c *Cluster) GetConsoleIpPortFromByType(proxyType string) (string, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	var proxyPort string
	switch proxyType {
	case "gotty":
		proxyPort = c.gottyPort
		break
	case "apiserver":
		proxyPort = c.apiServerProxyPort
		break
	case "dashboard":
		proxyPort = c.dashBoardProxyPort
		break
	case "gateway":
		proxyPort = c.gatewayManagePort
		break
	}
	for _, kn := range c.KubeNodes {
		if kn.Type == KubeNodeType_Console {
			if kn.RealHost != "" && kn.RealHost != "*" {
				proxyIPPort := kn.RealHost + ":" + proxyPort
				return proxyIPPort, nil
			}
		}
	}

	return "", errors.New("can not find proxy server in cluster [" + c.ClusterName + "]")
}

func (c *Cluster) GetDashboardIpPort() (string, error) {
	var ipPort string = ""
	var err error = nil

	//从console中获取apiserver地址，如果网络模式是vpc lb，则默认同时支持lb模式访问dashboard
	ipPort, err = c.GetConsoleIpPortFromByType("dashboard")

	if err != nil {
		return "", err
	}

	dashboardIPPort := "https://" + ipPort
	return dashboardIPPort, nil
}

func (c *Cluster) GetApiServerIpPort() (string, error) {
	var ipPort string = ""
	var err error = nil
	ipPort, err = c.GetConsoleIpPortFromByType("apiserver")
	if err != nil {
		return "", err
	}
	return "https://" + ipPort, nil
}

func (c *Cluster) GetGottyIpPort() (string, error) {
	var ipPort string = ""
	var err error = nil
	ipPort, err = c.GetConsoleIpPortFromByType("gotty")
	if err != nil {
		return "", err
	}
	return "http://" + ipPort, nil
}

func (c *Cluster) SetOriginalMsg(msg string) {
	c.originalMsg = msg
}

//GetOriginalMsg
func (c *Cluster) GetOriginalMsg() string {
	return c.originalMsg
}

//PrintClusterDebug 打印集群的debug信息
func (c *Cluster) PrintClusterDebug() {
	fmt.Printf("Cluster: %s\n", c.ClusterName)
	for _, node := range c.KubeNodes {
		node.PrintNodeDebug()
	}
}

//GetFSM 获取集群状态机
func (c *Cluster) GetFSM() *FSM {
	return c.k8sFsm
}

//GetApiServerBackends 获取cluster集群中master节点上bridge到宿主机上的apiserver地址
func (c *Cluster) GetApiServerBackends() []string {
	apiServers := []string{}
	for _, node := range c.KubeNodes {
		if node.Type == KubeNodeType_Master {
			apiServers = append(apiServers, (node.RealHost + ":6443"))
		}
	}
	return apiServers
}

//设置生成gotty、apiserver、dashboard、gateway端口
func (c *Cluster) updateConsolePorts() {
	beginPort := 40000
	endPort := 65535
	//暂时使用随机生成的端口
	rand.Seed(time.Now().UnixNano())
	c.gottyPort = strconv.Itoa(rand.Intn(endPort-beginPort) + beginPort)
	rand.Seed(time.Now().UnixNano())
	c.apiServerProxyPort = strconv.Itoa(rand.Intn(endPort-beginPort) + beginPort)
	rand.Seed(time.Now().UnixNano())
	c.dashBoardProxyPort = strconv.Itoa(rand.Intn(endPort-beginPort) + beginPort)
	rand.Seed(time.Now().UnixNano())
	c.gatewayManagePort = strconv.Itoa(rand.Intn(endPort-beginPort) + beginPort)
	log.Infof("generate console ports, gotty: %s,  apiserver: %s,  dashboard: %s,  gatewayManage: %s",
		c.gottyPort, c.apiServerProxyPort, c.dashBoardProxyPort, c.gatewayManagePort)
	//TODO需要结合mesos资源情况选择端口
	c.Conf.PutVariable("GOTTY_PORT", &c.gottyPort)
	c.Conf.PutVariable("API_PROXY_PORT", &c.apiServerProxyPort)
	c.Conf.PutVariable("DASH_PROXY_PORT", &c.dashBoardProxyPort)
	c.Conf.PutVariable("MANAGE_PORT", &c.gatewayManagePort)
	//更新存储cluster的conf
	c.storageCli.StoreConf(c)
}

//设置生成gotty、apiserver、dashboard、gateway端口
func (c *Cluster) updateProxyRoute() {
	var consoleNode *KubeNode
	var runningNodeIps = make([]string, 0)
	for _, node := range c.KubeNodes {
		if node.Type == KubeNodeType_Console {
			consoleNode = node
		}
		//如果node节点的状态为run才将其作为proxy gateway备选节点
		if node.Type == KubeNodeType_Node && node.Expect == cc.ExpectRUN {
			runningNodeIps = append(runningNodeIps, node.NodeIP)
		}
	}
	if consoleNode == nil || consoleNode.Expect == cc.ExpectSTOP {
		return
	}
	url := fmt.Sprintf("http://%s:%s/gateway", consoleNode.RealHost, c.gatewayManagePort)
	data, err := json.Marshal(runningNodeIps)
	if err != nil {
		log.Errorf("update proxy route error, msg: %s", err.Error())
	}
	body := bytes.NewReader(data)
	_, err = http.Post(url, "application/json", body)
	if err != nil {
		log.Errorf("update proxy route error, msg: %s", err.Error())
	}

}
