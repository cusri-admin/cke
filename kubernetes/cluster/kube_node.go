package cluster

import (
	cc "cke/cluster"
	"cke/kubernetes"
	"cke/kubernetes/cluster/conf"
	"cke/log"
	"cke/utils"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coreos/etcd/pkg/types"
)

//监听task事件
type ListenType int

var (
	Listen_Started ListenType = 0
	Listen_Stopped ListenType = 1
)

//KubeNodeType 标识节点的类型
type KubeNodeType string

//各种node类型
const (
	KubeNodeType_Master  KubeNodeType = "master"
	KubeNodeType_Node    KubeNodeType = "node"
	KubeNodeType_Console KubeNodeType = "console"
)

//MarshalJSON 将KubeNodeType编码为字符串
func (kt *KubeNodeType) MarshalJSON() ([]byte, error) {
	return json.Marshal(*kt)
}

//UnmarshalJSON 从字符串解码KubeNodeType
func (kt *KubeNodeType) UnmarshalJSON(b []byte) error {
	var state string
	json.Unmarshal(b, &state)
	switch KubeNodeType(strings.ToLower(state)) {
	case KubeNodeType_Master:
		*kt = KubeNodeType_Master
		break
	case KubeNodeType_Node:
		*kt = KubeNodeType_Node
		break
	case KubeNodeType_Console:
		*kt = KubeNodeType_Console
		break
	}
	return nil
}

//KubeNode k8s node数据结构
type KubeNode struct {
	Name      string       `json:"name"`
	Type      KubeNodeType `json:"type"`    //值为 "master", "node"和"console"三种
	Host      string       `json:"host"`    //kubenode的物理主机约束
	NodeIP    string       `json:"node_ip"` //kubenode在contiv网络中的具体ip
	Processes []*Process   `json:"process"` //该节点上的运行的系统进程
	//Ingresses   []string                           `json:"ingresses,omitempty"`  //节点开放的ingress端口
	Ingress     bool                               `json:"ingress,omitempty"`    //是否启用ingress
	Task        *cc.Task                           `json:"-"`                    //这个节点(wrapper)所对应的task
	Attributes  []*Attribute                       `json:"attributes,omitempty"` //自定义属性
	StartTime   time.Time                          `json:"start_time"`           //启动时间
	Expect      cc.ExpectStatus                    `json:"expect"`               //该节点的期望运行状态
	RealHost    string                             //实际运行的节点
	fsm         *FSM                               //节点状态机，分为两种master节点共用一个状态机，node几点各自拥有状态机
	cluster     *Cluster                           //所属集群的指针
	listenChans map[ListenType]([]chan ListenType) //用于监听node的wrapper task的状态变化
	Conf        *conf.NodeConf                     //该节点的配置信息
	NodeID      string                             //节点ID
	Reservation *cc.Reservation                    `json:"reservation"` //主动资源预留 该项为nil时,在host中不能使用*
}

//Initialize 初始化节点
func (k *KubeNode) Initialize(cluster *Cluster) error {
	k.Conf = cluster.Conf.GetNodeConf(string(k.Type))
	if k.Conf == nil {
		return errors.New("Configuration with the node type " + string(k.Type) + " could not be found")
	}
	k.Conf.PutVariable("NODE_IP", &k.NodeIP)
	//随机生成k8s中pod网段
	if k.Type == KubeNodeType_Master {
		ipNum := rand.Intn(220) + 30 //calico网段从 172.30至172.250
		clusterPodCIDR := fmt.Sprintf("172.%d.0.0/16", ipNum)
		k.Conf.PutVariable("ClusterPodCidr", &clusterPodCIDR)
	}

	k.Expect = cc.ExpectRUN
	k.RealHost = k.Host
	k.cluster = cluster
	k.StartTime = time.Now()
	for _, p := range k.Processes {
		if err := p.Initialize(k); err != nil {
			return err
		}
	}
	return k.checkFormat()
}

//GetReservation 获得该节点希望预留的资源信息
func (k *KubeNode) GetReservation() *cc.Reservation {
	if k.Reservation.IsNeedOperation && !k.Reservation.PreReserved {
		return k.Reservation
	}
	return nil
}

//ReleaseResource 释放该节点预留的资源
func (k *KubeNode) ReleaseResource() {
	if !k.Reservation.PreReserved {
		k.Reservation.Target = &cc.Resource{
			CPU: 0,
			Mem: 0,
		}
		wg := new(sync.WaitGroup)
		wg.Add(1)
		k.Reservation.IsNeedOperation = true
		k.Reservation.Callback = func() {
			//及时更新storage中的节点reservation信息，防止leader切换
			k.cluster.storageCli.StoreReservation(k, k.Reservation)
			wg.Done()
		}
		//及时更新storage中的节点reservation信息，防止leader切换
		k.cluster.storageCli.StoreReservation(k, k.Reservation)
		log.Debugf("Releaseing for node %s unreservation", k.Reservation.ReservationID)
		wg.Wait()
		log.Debugf("Released resource of node %s", k.Reservation.ReservationID)
	}
}

//GetResource 获得该Node使用的资源总量
func (k *KubeNode) GetResource() *cc.Resource {
	res := &cc.Resource{}
	//短任务只需要得到最大资源即可
	//即该资源可以运行任何一个任务
	maxRes := &cc.Resource{
		CPU:  0,
		Mem:  0,
		Disk: 0,
	}
	for _, p := range k.Processes {
		if p.Expect == cc.ExpectRUN {
			res.AddResource(p.Res)
		} else {
			if maxRes.CPU < p.Res.CPU {
				maxRes.CPU = p.Res.CPU
			}
			if maxRes.Mem < p.Res.Mem {
				maxRes.Mem = p.Res.Mem
			}
			if maxRes.Disk < p.Res.Disk {
				maxRes.Disk = p.Res.Disk
			}
		}
	}
	//添加短任务的资源
	res.AddResource(maxRes)
	//添加Exector和Wrapper的资源
	//wrapper
	res.AddResource(k.Task.Info.Res)
	//Executor
	res.AddResource(k.Task.Info.Executor.Res)
	//多增加一个，因为最后一个任务需要配置Executor
	res.AddResource(k.Task.Info.Executor.Res)
	return res
}

func (k *KubeNode) getUsedResource() (*cc.Resource, bool) {
	res := &cc.Resource{}
	for _, pro := range k.Processes {
		if pro.Task.Status != nil {
			if pro.Task.Status.State == cc.TASKMESOS_STAGING ||
				pro.Task.Status.State == cc.TASKMESOS_STARTING ||
				pro.Task.Status.State == cc.TASKMESOS_KILLING {
				return nil, false
			}
			if pro.Task.Status.State == cc.TASKMESOS_RUNNING {
				res.AddResource(pro.Res)
			}
		}
	}
	if k.Task.Status != nil {
		if k.Task.Status.State == cc.TASKMESOS_STAGING ||
			k.Task.Status.State == cc.TASKMESOS_STARTING ||
			k.Task.Status.State == cc.TASKMESOS_KILLING {
			return nil, false
		}
		if k.Task.Status.State == cc.TASKMESOS_RUNNING {
			res.AddResource(k.Task.Info.Res)
			res.AddResource(k.Task.Info.Executor.Res)
		}
	}
	return res, true
}

// 生成运行KubeNode(Docker container)的任务,该任务由K8s executor解析执行
func (k *KubeNode) generateK8sTask(cluster *Cluster) error {
	if k.Reservation == nil {
		k.Reservation = &cc.Reservation{
			Host:            &k.RealHost,
			ReservationID:   cluster.ClusterName + "." + k.Name,
			GetUsedResource: k.getUsedResource,
			PreReserved:     false,
			Current: &cc.Resource{
				CPU: 0,
				Mem: 0,
			},
		}
	}
	k.NodeID = cluster.ClusterName + "-" + k.Name + "-container"
	k8sTask := &kubernetes.K8STask{
		NodeId:   k.NodeID,
		TaskType: kubernetes.K8STaskType_START_NODE,
		Image:    k.Conf.GetImagePath(),
		Node: &kubernetes.K8SNodeInfo{
			Network:   cluster.Network,
			Ip:        k.NodeIP,
			DnsRecord: cluster.getDNSRecord(), //该k8s集群的master dns信息(为IP列表，master name固定为master.<集群名>.cke)
		},
	}

	//定义node的可见端口
	if k.Type == KubeNodeType_Console {
		//设置console节点上端口
		cluster.updateConsolePorts()
		k8sTask.Node.PublishPorts = []string{
			cluster.gottyPort + ":" + cluster.gottyPort,
			cluster.apiServerProxyPort + ":" + cluster.apiServerProxyPort,
			cluster.dashBoardProxyPort + ":" + cluster.dashBoardProxyPort,
			cluster.gatewayManagePort + ":" + cluster.gatewayManagePort}
	} else {
		//if len(k.Ingresses) > 0 {
		if k.Ingress {
			//ingressPorts := cluster.Ingresses.GetPortsByNames(k.Ingresses)
			ingressPorts := cluster.Ingress.GetIngressPorts()
			k8sTask.Node.PublishPorts = make([]string, 0, len(ingressPorts))
			for _, ingPort := range ingressPorts {
				k8sTask.Node.PublishPorts = append(k8sTask.Node.PublishPorts, string(ingPort)+":"+string(ingPort))
				log.Infof("generateK8sTask PublishPorts.%s", string(ingPort)+":"+string(ingPort))
			}

		}
	}

	taskInfo := &cc.TaskInfo{
		Name: k8sTask.GetNodeId(),
		Type: "k8s",
		Host: &k.RealHost,
		Body: k8sTask,
		Res: &cc.Resource{
			CPU: 0.1,
			Mem: 64,
		},
		Executor:    getExecutorInfo(cluster.ClusterName, k.Name),
		State:       &k.Expect,
		Reservation: k.Reservation,
	}

	k.Task = &cc.Task{
		Info: taskInfo,
	}

	//处理该节点下的进程
	// 添加该KubeNode中的扩展process
	if k.Type == KubeNodeType_Master {
		//增加etcdHc process
		etcdHc := Process_ETCDHC.CreateProcess()
		k.Processes = append(k.Processes, etcdHc)
		mastercalicoInit := Process_INITMASTERCALICO.CreateProcess()
		k.Processes = append(k.Processes, mastercalicoInit)
	} else if k.Type == KubeNodeType_Node {
		//增加kube node config process
		nodeConfig := Process_NODECONF.CreateProcess()
		k.Processes = append(k.Processes, nodeConfig)
		//增加createCLBuildingProcess
		clBuilding := Process_CRBUILDING.CreateProcess()
		k.Processes = append(k.Processes, clBuilding)
		//增加kube calico init process
		calicoInit := Process_INITNODECALICO.CreateProcess()
		k.Processes = append(k.Processes, calicoInit)
		//增加createKubeletHcProcess
		kubeletHc := Process_KUBELETHC.CreateProcess()
		k.Processes = append(k.Processes, kubeletHc)
		//初始化添加移除几点时要执行的process，当有移除节点请求才执行
		removeNode := Process_REMOVENODE.CreateProcess()
		k.Processes = append(k.Processes, removeNode)
	} else if k.Type == KubeNodeType_Console {
		//增加kubectl config process
		kubectlConfig := Process_KUBECTLCONF.CreateProcess()
		k.Processes = append(k.Processes, kubectlConfig)
	}

	// 生成该KubeNode中的每个process的任务
	for _, process := range k.Processes {
		log.Debugf("Node %s add process: %s type: %s", k.Name, process.Name, process.Type.String())
		err := process.generateK8sTask(cluster, k)
		if err != nil {
			log.Error("KubeNode generateK8sTask error : ", err)
			return err
		}
	}

	if !k.Reservation.PreReserved {
		//生成各子任务后计算总资源使用量
		k.Reservation.Target = k.GetResource()
		k.Reservation.IsNeedOperation = true
		k.Reservation.Callback = func() {
			//及时更新storage中的资源预留信息，防止leader切换
			k.cluster.storageCli.StoreReservation(k, k.Reservation)
		}
	}
	return nil
}

func (k *KubeNode) isNeedSchedule() bool {
	if k.Expect != cc.ExpectRUN {
		return false
	}
	if k.Task == nil {
		return false
	}
	if k.Task.Status == nil { //unKnown 状态
		return true
	}
	if k.Task.Status.State.Int() > k.Task.Info.State.Int() {
		return true
	}
	return false
}

//判断node wrapper task是否启动成功
func (k *KubeNode) isStarted() bool {
	if k.Task != nil && k.Task.Status != nil && k.Task.Status.State == cc.TASKMESOS_RUNNING {
		return true
	}
	return false
}

//判断node wrapper task是否被停止成功
func (k *KubeNode) isStopped() bool {
	if *k.Task.Info.State == cc.ExpectSTOP && k.Task.Status.State != cc.TASKMESOS_RUNNING {
		return true
	}
	return false
}

//GetInconsistentTasks 获得该节点及其子进程需要执行的task
func (k *KubeNode) GetInconsistentTasks(procesTypes types.Set) ([]*cc.TaskInfo, bool) {
	taskList := make([]*cc.TaskInfo, 0, len(k.Processes))
	//先判断申请的资源是否具备
	if !k.Reservation.PreReserved && k.Reservation.IsNeedOperation && k.Expect != cc.ExpectSTOP {
		return taskList, false
	}

	if k.Type == KubeNodeType_Master && procesTypes.Contains(Process_MASTERWRAP.String()) ||
		k.Type == KubeNodeType_Node && procesTypes.Contains(Process_NODEWRAP.String()) ||
		k.Type == KubeNodeType_Console && procesTypes.Contains(Process_CONSOLEWRAP.String()) {
		//规划启动master wrapper  或者  node wrapper 或者 console wrapper
		if k.isStarted() {
			return taskList, true
		}
		if k.isNeedSchedule() {
			//*k.Task.Info.Host = k.Host //保证每个kube node wrapper启动都为用户定义host
			taskList = append(taskList, k.Task.Info)
		}
		return taskList, false
	}
	//规划启动process
	isComplete := true
	if k.Processes != nil {
		for _, pro := range k.Processes {
			proTask, isCom := pro.GetInconsistentTasks(procesTypes)
			if !isCom {
				if proTask != nil {
					taskList = append(taskList, proTask)
				}
				isComplete = false
			}
		}
	}
	return taskList, isComplete
}

func (k *KubeNode) mayRollBack(status *cc.TaskStatus, cluster *Cluster) {

	if k.isNeedSchedule() {
		//清空子任务的状态，以便重新执行
		for _, process := range k.Processes {
			process.clearStatus()
		}

		//更新kubenode中所有任务status置空存储到的storage中，防止leader切换后任务执行信息丢失
		for _, process := range k.Processes {
			process.Task.Status = nil
		}

		cluster.storageCli.StoreKubeNode(k)

		if k.Type == KubeNodeType_Master {
			k.fsm.Call(NewFsmEvent(&RollBackType, Process_MASTERWRAP))
		} else if k.Type == KubeNodeType_Node {
			k.fsm.Call(NewFsmEvent(&RollBackType, Process_NODEWRAP))
		} else {
			k.fsm.Call(NewFsmEvent(&RollBackType, Process_CONSOLEWRAP))
		}
	}
}

//UpdateTaskStatus 更新任务状态
//在这个方法里判断如果该节点是console并且启动失败了，将gotty端口+1
func (k *KubeNode) UpdateTaskStatus(status *cc.TaskStatus, cluster *Cluster) bool {
	if k.Task.Info.Name == status.TaskName {
		k.Task.Status = status
		k.mayRollBack(status, cluster)
		k.notifyListenner()
		if status.State == cc.TASKMESOS_RUNNING {
			k.StartTime = time.Now()
		} else {
			if k.Type == KubeNodeType_Console {
				if status.State == cc.TASKMESOS_FAILED ||
					status.State == cc.TASKMESOS_LOST {
					k.updateConsolePorts(cluster)
				}
			}
		}
		// TODO 缺少更新etcd的代码
		return true
	}
	for _, process := range k.Processes {
		if process.UpdateTaskStatus(status, k, cluster) {
			return true
		}
	}
	return false
}

//StopKubeNode 停止节点
func (k *KubeNode) StopKubeNode(cluster *Cluster) {
	k.Expect = cc.ExpectSTOP
	if k.Task.Status != nil &&
		(k.Task.Status.State == cc.TASKMESOS_RUNNING ||
			k.Task.Status.State == cc.TASKMESOS_STARTING) {
		for _, pro := range k.Processes {
			pro.StopTask(cluster)
		}
		cluster.sch.StopTask(cluster, k.Task)
	}
}

//StopProcess 停止进程
func (k *KubeNode) StopProcess(cluster *Cluster, proName string) (bool, *Process) {
	for _, pro := range k.Processes {
		if pro.Name == proName {
			pro.StopTask(cluster)
			return true, pro
		}
	}
	return false, nil
}

func (k *KubeNode) removeProcessFromList(proName string) bool {
	for idx, pro := range k.Processes {
		if pro.Name == proName {
			if (idx + 1) < len(k.Processes) {
				k.Processes = append(k.Processes[:idx], k.Processes[idx+1:]...)
			} else {
				k.Processes = k.Processes[:idx]
			}
			return true
		}
	}
	return false
}

//UpdateProcess 更新进程
func (k *KubeNode) UpdateProcess(cluster *Cluster, processToUpdate []*Process) error {

	if k.Expect != cc.ExpectRUN {
		return errors.New("Can't update process to stopping node")
	}

	proToUpdMap := make(map[string]*Process)
	stoppingProcess := make([]*Process, 0, len(processToUpdate))
	stoppingProChan := make(map[string]<-chan ListenType)
	for _, proToUpdate := range processToUpdate {
		if isStopping, pro := k.StopProcess(cluster, proToUpdate.Name); isStopping {
			stoppingProcess = append(stoppingProcess, pro)
			proToUpdMap[proToUpdate.Name] = proToUpdate
			stoppingProChan[pro.Name] = pro.RegistListener(Listen_Stopped)
		}
	}
	//等待旧process停止，重新生成process及task, 并向fsm发送回滚事件
	go func() {
		for _, pro := range stoppingProcess {
			<-stoppingProChan[pro.Name]
		}
		for _, pro := range stoppingProcess {
			//如果停止成功,以新参数重新生成process任务
			proToUpdMap[pro.Name].generateK8sTask(cluster, k)
			k.removeProcessFromList(pro.Name)
			k.Processes = append(k.Processes, proToUpdMap[pro.Name])
			cluster.storageCli.StoreProcess(k, proToUpdMap[pro.Name])
		}
		if !k.Reservation.PreReserved {
			//等待修改资源预留
			wg := new(sync.WaitGroup)
			wg.Add(1)
			k.Reservation.Target = k.GetResource()
			k.Reservation.IsNeedOperation = true
			k.Reservation.Callback = func() {
				//及时更新storage中的节点reservation信息，防止leader切换
				cluster.storageCli.StoreReservation(k, k.Reservation)
				wg.Done()
			}
			//及时更新storage中的节点reservation信息，防止leader切换
			cluster.storageCli.StoreReservation(k, k.Reservation)
			wg.Wait()
			log.Debugf("Update resource of node %s", k.Reservation.ReservationID)
		}
		for _, pro := range stoppingProcess {
			//产生rollback事件
			k.fsm.Call(NewFsmEvent(&RollBackType, pro.Type))
		}
	}()
	return nil
}

func (k *KubeNode) getProcessStdFile(processName string, fileName string,
	offset uint64, length uint64) ([]byte, error) {
	for _, p := range k.Processes {
		if p.Name == processName {
			return p.getStdFile(k.Task.Info.Name, fileName, offset, length)
		}
	}
	return nil, utils.NewHttpError(http.StatusNotFound, "process "+processName+" could not be found in "+k.Name)
}

//GetIngresses 获得该节点所有当前的Ingress
/*func (k *KubeNode) GetIngresses() []*Ingress {
	ingresses := []*Ingress{}
	for _, name := range k.Ingresses {
		ingresses = append(ingresses, k.cluster.Ingresses.GetIngress(name))
	}
	return ingresses
}*/

func (k *KubeNode) GetIngresses() []IngressPort {
	if k.Ingress {
		return k.cluster.Ingress.GetIngressPorts()
	}
	return nil
}

//CheckFormat 检查KubeNode的合法性
func (k *KubeNode) checkFormat() error {
	if len(k.Name) <= 0 {
		return errors.New("KubeNode name is null")
	}

	//检查pre_reserved的合法性
	if k.Reservation != nil {
		if k.Reservation.PreReserved {
			if k.Host == "*" || len(k.Host) <= 0 {
				return errors.New("KubeNode " + k.Name + " host must net be * or nil when reservation.pre_reserved is true")
			}
		} else {
			return errors.New("KubeNode " + k.Name + " reservation.pre_reserved must be true")
		}
	}

	/*if len(k.Ingresses) > 0 {
		for _, name := range k.Ingresses {
			if len(name) <= 0 {
				return errors.New(
					"ingress name is null in KubeNode " + k.Name)
			}
			if k.cluster.Ingresses == nil {
				return errors.New(
					"Can't find ingress by name \"" + name + "\" in KubeNode " + k.Name + ", the definition of ingresses is nil. ")
			}
			ig := k.cluster.Ingresses.GetIngress(name)
			if ig == nil {
				return errors.New(
					"Can't find ingress by name \"" + name + "\" in KubeNode " + k.Name)
			}
		}
	}*/
	return nil
}

//删除从storage中获取到的老的集群任务，包括 finished 的任务及 长运行任务状态的状态
func (k *KubeNode) trimOldInfo() error {
	k.Task = nil
	for i := 0; i < len(k.Processes); {
		if k.Processes[i].Task != nil && *k.Processes[i].Task.Info.State == cc.ExpectFINISHED {
			k.Processes = append(k.Processes[:i], k.Processes[i+1:]...)
		} else {
			k.Processes[i].trimOldInfo()
			i++
		}
	}
	return nil
}

func (k *KubeNode) initialFSM(cluster *Cluster, relatedNodes []*KubeNode) {

	nodes := make([]*KubeNode, 0)
	nodes = append(nodes, k)
	nodes = append(nodes, relatedNodes...)
	if k.Type == KubeNodeType_Master {
		if k.fsm != nil {
			k.fsm.ReleatedNodes = relatedNodes
		}
	} else if k.Type == KubeNodeType_Node {
		k.fsm = NewFSM("node", &NodeWrapState, cluster.publisher, cluster, nodes) //创建node类型状态机
		cluster.k8sFsm.addChild(k.fsm)
	} else {
		k.fsm = NewFSM("console", &ConsoleWrapState, cluster.publisher, cluster, nodes) //创建console类型状态机
		cluster.k8sFsm.addChild(k.fsm)
	}
}

//从k8s集群中清除该节点的痕迹
func (k *KubeNode) clearNodeFromK8s(cluster *Cluster) <-chan ListenType {
	if k.Type == KubeNodeType_Master {
		return nil
	}
	k.Expect = cc.ExpectSTOP

	var removeNodeProcess *Process
	for _, process := range k.Processes {
		if process.Type == Process_REMOVENODE {
			removeNodeProcess = process
			break
		}
	}

	//如果kube node的名称和类型与在集群中存在则删除
	deletingNodes := map[string]*KubeNode{k.Name: k}
	removeNodeProcess.generateK8sTask(cluster, k, deletingNodes)
	//cluster.storageCli.StoreTask(k, removeNodeProcess, removeNodeProcess.Task)
	cluster.storageCli.StoreKubeNode(k)
	if k.isStarted() {
		k.fsm.Call(NewFsmEvent(&RemoveNodeType))
		return removeNodeProcess.RegistListener(Listen_Started)
	}
	//向process中注册监听信道，监听任务启动成功或者执行完成后通过信道通知执行后续任务

	return nil
}

func (k *KubeNode) notifyListenner() {
	if k.listenChans == nil {
		return
	}
	//如果出现”停止“事件则通知相应chan
	if k.isStopped() {
		for _, ch := range k.listenChans[Listen_Stopped] {
			ch <- Listen_Stopped
		}
		k.listenChans[Listen_Stopped] = make([]chan ListenType, 0)
	}

	//如果出现”运行“事件则通知相应chan
	if k.isStarted() {
		for _, ch := range k.listenChans[Listen_Started] {
			ch <- Listen_Started
		}
		k.listenChans[Listen_Started] = make([]chan ListenType, 0)
	}
}

//RegistListenner 注册listenner
func (k *KubeNode) RegistListenner(listenType ListenType) <-chan ListenType {
	if k.listenChans == nil {
		k.listenChans = make(map[ListenType][]chan ListenType)
	}

	if k.listenChans[listenType] == nil {
		k.listenChans[listenType] = make([]chan ListenType, 0)
	}
	listenChan := make(chan ListenType)
	k.listenChans[listenType] = append(k.listenChans[listenType], listenChan)

	return listenChan

}

func (k *KubeNode) updateConsolePorts(cluster *Cluster) {
	cluster.updateConsolePorts()
	k8sTask := k.Task.Info.Body.(*kubernetes.K8STask)
	k8sTask.Node.PublishPorts = []string{
		cluster.gottyPort + ":" + cluster.gottyPort,
		cluster.apiServerProxyPort + ":" + cluster.apiServerProxyPort,
		cluster.dashBoardProxyPort + ":" + cluster.dashBoardProxyPort,
		cluster.gatewayManagePort + ":" + cluster.gatewayManagePort}

	for _, process := range k.Processes {
		if process.Type == Process_GOTTY {
			if err := process.generateK8sTask(cluster, k); err != nil {
				log.Error("KubeNode generateK8sTask error: ", err)
			}
			break
		}
	}
}

//PrintNodeDebug 打印节点的debug信息
func (k *KubeNode) PrintNodeDebug() {
	fmt.Printf("  Nnode: %s, type: %s, Host: %s, IP: %s\n", k.Name, k.Type, k.RealHost, k.NodeIP)
	for _, proc := range k.Processes {
		proc.PrintProcessDebug()
	}
}
