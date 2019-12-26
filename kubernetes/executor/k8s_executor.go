package executor

import (
	"bufio"
	"errors"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"cke/executor"
	"cke/kubernetes"
	"cke/log"
	"cke/utils"

	"github.com/golang/protobuf/proto"
)

var (
	totalMem  int64
	totalCpus float64
)

func init() {
	totalCpus = float64(runtime.NumCPU())

	//例子 MemTotal:       98772860 kB
	fi, _ := os.Open("/proc/meminfo")
	// if err != nil {
	// 	log.Println("Read file /proc/meminfo error: " + err.Error())
	// 	totalMem = -1
	// 	return
	// }
	defer fi.Close()

	br := bufio.NewReader(fi)
	for {
		a, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}
		fields := strings.Fields(string(a))
		if fields[0] == "MemTotal:" {
			totalMem, _ = strconv.ParseInt(fields[1], 0, 64)
			break
		}
	}
}

//KubeNodeManager 管理KubeNode的接口
type KubeNodeManager interface {
	getStartingKubeNode(string) *KubeNodeContainer
	moveToRunningKubeNode(string)
	getRunningKubeNode(string) *KubeNodeContainer
}

//Executor kubernetes executor
type Executor struct {
	kubeNodeLock      sync.RWMutex
	caller            executor.Caller
	wrapperServer     *WrapperServer
	kubeNodes         map[string]*KubeNodeContainer
	stratingKubeNodes map[string]*KubeNodeContainer
	workPath          string
	fwID              string
	dockerCfs         bool
	dockerPath        string
	lxcfsPath         *string
	exitFunc          func(int)
}

//NewExecutor 创建一个K8S Executor
func NewExecutor(fwID string, workPath string, socketPath string,
	cfs bool, dockerPath string, lxcfsPath *string, exitFunc func(int)) (*Executor, error) {

	wrapperServer, err := createServer(socketPath)
	if err != nil {
		return nil, err
	}
	err = wrapperServer.start()
	if err != nil {
		return nil, err
	}
	log.Info("Started, server path:" + wrapperServer.serverPath)
	e := &Executor{
		wrapperServer:     wrapperServer,
		kubeNodes:         make(map[string]*KubeNodeContainer),
		stratingKubeNodes: make(map[string]*KubeNodeContainer),
		workPath:          workPath,
		fwID:              fwID,
		dockerCfs:         cfs,
		dockerPath:        dockerPath,
		lxcfsPath:         lxcfsPath,
		exitFunc:          exitFunc,
	}
	wrapperServer.kubeNodeManager = e
	return e, nil
}

func (e *Executor) getStartingKubeNode(nodeID string) *KubeNodeContainer {
	e.kubeNodeLock.RLock()
	defer e.kubeNodeLock.RUnlock()
	return e.stratingKubeNodes[nodeID]
}

func (e *Executor) getRunningKubeNode(nodeID string) *KubeNodeContainer {
	e.kubeNodeLock.RLock()
	defer e.kubeNodeLock.RUnlock()
	return e.kubeNodes[nodeID]
}

func (e *Executor) addStartingKubeNode(nodeID string, kubeNode *KubeNodeContainer) {
	e.kubeNodeLock.Lock()
	defer e.kubeNodeLock.Unlock()
	e.stratingKubeNodes[nodeID] = kubeNode
	log.Debugf("KubeNode %s registering", nodeID)
}

func (e *Executor) removeStartingKubeNode(nodeID string) {
	e.kubeNodeLock.Lock()
	defer e.kubeNodeLock.Unlock()
	delete(e.stratingKubeNodes, nodeID)
}

func (e *Executor) moveToRunningKubeNode(nodeID string) {
	e.kubeNodeLock.Lock()
	defer e.kubeNodeLock.Unlock()
	kubeNode := e.stratingKubeNodes[nodeID]
	if kubeNode != nil {
		delete(e.stratingKubeNodes, nodeID)
		e.kubeNodes[nodeID] = kubeNode
		log.Debugf("KubeNode %s registered", nodeID)
	}
}

func (e *Executor) startKubeNode(
	mesosTaskID string,
	kebuNodeID string,
	kubeNodeImage string,
	nodeInfo *kubernetes.K8SNodeInfo,
	cpus float64,
	mem float64) (*KubeNodeContainer, error) {

	kubeNode, err := CreateContainer(
		mesosTaskID,
		e.fwID,
		kebuNodeID,
		kubeNodeImage,
		utils.AppendPath(e.workPath, e.wrapperServer.getServerPath()),
		e.workPath,
		nodeInfo,
		e.caller,
		e.dockerCfs,
		e.dockerPath,
		e.lxcfsPath,
		cpus,
		mem)
	if err != nil {
		return nil, errors.New("Create KubeNode err:" + err.Error())
	}
	e.addStartingKubeNode(kebuNodeID, kubeNode)

	err = kubeNode.StartContainer(e.kubeNodeConnected)
	if err != nil {
		e.removeStartingKubeNode(kebuNodeID)
		return nil, err
	}
	return kubeNode, nil
}

func (e *Executor) kubeNodeConnected(kubeNodeID string) {
	//已经连接
	kubeNode := e.getRunningKubeNode(kubeNodeID)
	e.caller.CallTaskRunning(kubeNode.taskID)

	go func(kubeNodeID string) {
		statusCode, err := kubeNode.WaitingExited()
		if err != nil {
			log.Error("Run KubeNode err:" + err.Error())
		}
		e.kubeNodeLock.Lock()
		delete(e.kubeNodes, kubeNodeID)
		e.kubeNodeLock.Unlock()
		log.Info("KubeNode(" + kubeNodeID + ") container(" + kubeNode.containerID + ") exit code:" + strconv.FormatInt(statusCode, 10))

		e.exitFunc(0)
	}(kubeNodeID)
	log.Info("Started KubeNode id:" + kubeNodeID + " container:" + kubeNode.containerID)
}

//UnmarshalTaskBody 接口实现由framework.executor调用
func (e *Executor) UnmarshalTaskBody(taskBody []byte) (interface{}, error) {
	task := &kubernetes.K8STask{}
	err := proto.Unmarshal(taskBody, task)
	if err != nil {
		return nil, err
	}
	return task, nil
}

//Started 回调函数由framework.executor
func (e *Executor) Started() {

}

//Registered 回调函数由framework.executor回调
func (e *Executor) Registered(caller executor.Caller) {
	e.caller = caller
}

//Launch 启动一个k8s的任务
// 任务包括两种类型：启动容器的任务，启动容器内进程的任务
func (e *Executor) Launch(mesosTaskID string, task *executor.TaskInfoWithResource) error {
	//task := taskInfo.(*K8sTask)
	//task := taskInfo.(*executor.TaskInfoWithResource)
	k8sTask := task.Task.(*kubernetes.K8STask)
	if log.GetLevel() >= log.INFO {
		kubernetes.PrintK8STask(k8sTask)
	}

	kubeNode := e.getRunningKubeNode(k8sTask.NodeId)
	if k8sTask.GetTaskType() == kubernetes.K8STaskType_START_NODE { //是启动容器的任务
		if kubeNode != nil { //该Node已经存在了
			err := errors.New("KubeNode " + k8sTask.NodeId + " already exists.")
			log.Error("Launch error: " + err.Error())
			return err
		}
		e.caller.CallTaskStarting(mesosTaskID)
		var err error
		kubeNode, err = e.startKubeNode(
			mesosTaskID,
			k8sTask.GetNodeId(),
			k8sTask.GetImage(),
			k8sTask.GetNode(),
			task.Cpus,
			task.Mem)
		if err != nil {
			log.Error("Launch error: " + err.Error())
			e.caller.CallTaskFailed(mesosTaskID, err.Error())
			//启动出错了
			//TODO: 重构代码去掉多容器支持
			e.exitFunc(0)
		}
	} else { //是启动进程的任务
		if kubeNode == nil { //该Node还不存在
			err := errors.New("KubeNode " + k8sTask.NodeId + " does not exist.")
			log.Error("Launch error: " + err.Error())
			return err
		}
		//启动任务
		kubeNode.StartProcess(mesosTaskID, task)
	}
	return nil
}

//Kill Kill指定task
func (e *Executor) Kill(mesosTaskID string) error {
	log.Info("Received Kill Task: " + mesosTaskID)

	var kubeNode *KubeNodeContainer
	e.kubeNodeLock.Lock()
	for _, kn := range e.kubeNodes {
		if kn.isWrapperTask(mesosTaskID) {
			e.kubeNodeLock.Unlock()
			kn.Shutdown()
			return nil
		} else if kn.hasTask(mesosTaskID) {
			kubeNode = kn
			break
		}
	}
	e.kubeNodeLock.Unlock()

	if kubeNode != nil {
		kubeNode.StopProcess(mesosTaskID)
		return nil
	} else {
		//已经没有这个容器了，停止自己
		//TODO: 重构代码去掉多容器支持
		e.exitFunc(0)
	}
	err := errors.New("kill task error: can't found task: " + mesosTaskID)
	log.Error("Kill task error:", err.Error())
	return err
}

//Message 发送消息
func (e *Executor) Message(message []byte) {
	log.Debug("Received Message")
}

func (e *Executor) Error(err string) {
	log.Debug("Received Error: " + err)
}

//Shutdown Shutdown executor
func (e *Executor) Shutdown() {
	//并发发送showdown命令，停止各KubeNode
	var wg sync.WaitGroup
	e.kubeNodeLock.Lock()
	for nodeID, kubeNode := range e.kubeNodes {
		wg.Add(1)
		log.Infof("Executor.Shutdown KubeNode(%s)%s", nodeID, kubeNode.nodeID)
		go func(k *KubeNodeContainer) {
			k.Shutdown()
			wg.Done()
		}(kubeNode)
	}
	e.kubeNodeLock.Unlock()
	wg.Wait()

	e.wrapperServer.stop()

	if _, err := utils.RemoveDir(e.dockerPath); err != nil {
		log.Errorf("Remove Executor's docker working directory (%s) error: %s", e.dockerPath, err.Error())
	}
}
