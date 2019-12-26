package wrapper

import (
	"cke/kubernetes"
	"cke/log"
	"context"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
)

type runtimeInfo struct {
	info    *kubernetes.ProcessInfo
	process *os.Process
}

//K8sWrapper 胖容器的数据主体
type K8sWrapper struct {
	nodeID     string
	socketPath string
	stream     kubernetes.WrapperProto_EventCallClient
	runnings   map[string]*processRuntime
	eventChan  chan kubernetes.WrapperEvent
	outputPath string
	heartbeat  int32
	hbLock     sync.RWMutex
	hbTimer    *time.Timer
}

//CreateK8sWrapper 创建一个Wrapper
func CreateK8sWrapper(socketPath string, nodeID string, outputPath string, heartbeat int32) *K8sWrapper {
	return &K8sWrapper{
		nodeID:     nodeID,
		socketPath: socketPath,
		runnings:   make(map[string]*processRuntime),
		eventChan:  make(chan kubernetes.WrapperEvent),
		outputPath: outputPath,
		heartbeat:  heartbeat,
	}
}

//Run 启动gRPC客户端，并进入工作循环
func (kw *K8sWrapper) Run() error {
	conn, err := grpc.Dial("unix://"+kw.socketPath, grpc.WithInsecure())
	if err != nil {
		log.Errorf("Connection failed: [%v] ", err)
		return err
	}
	//defer conn.Close()
	// 声明客户端
	gRPCClient := kubernetes.NewWrapperProtoClient(conn)
	// 声明 context
	ctx := context.Background()
	// 创建双向数据流
	kw.stream, err = gRPCClient.EventCall(ctx)

	if err != nil {
		log.Errorf("Failed to create data stream: [%v] ", err)
		return err
	}

	// 向服务端发送注册 指令
	reg := &kubernetes.WrapperEvent{
		Type:   kubernetes.WrapperEvent_REGISTERED,
		NodeId: kw.nodeID,
	}
	if err := kw.stream.Send(reg); err != nil {
		log.Error("Register to executor error: " + err.Error())
		return err
	}
	log.Info("Begin event loop......")
	kw.resetHeartbeat()
	for {
		// 接收从 服务端返回的数据流
		execEvent, err := kw.stream.Recv()
		if err == io.EOF {
			log.Warning("wrapper stop，msg: ", err.Error())
			break //如果收到结束信号，则退出“接收循环”，结束客户端程序
		}
		if err != nil {
			// TODO: 处理接收错误
			log.Error("Error receiving data: ", err)
			kw.stopHeartbeat()
			return err
		}
		//没有错误的情况下，执行来自服务端的消息
		switch execEvent.Type {
		case kubernetes.ExecutorEvent_SAVE_FILE:
			//保存文件
			go func(execEvent *kubernetes.ExecutorEvent) {
				err := saveFiles(execEvent.Files)
				if err != nil {
					log.Errorf("save files error: %s", err.Error())
					kw.callExecutor(execEvent.TaskId, kubernetes.WrapperEventResponse_UNSAVED, err.Error())
				} else {
					kw.callExecutor(execEvent.TaskId, kubernetes.WrapperEventResponse_SAVED, "OK")
				}
			}(execEvent)
			break
		case kubernetes.ExecutorEvent_START_REQ:
			go kw.startProcess(execEvent.TaskId, execEvent.Process)
			break
		case kubernetes.ExecutorEvent_STOP_REQ:
			go kw.stopProcess(execEvent.TaskId)
			break
		case kubernetes.ExecutorEvent_MESSAGE:
			go kw.message(execEvent.Msg)
			break
		case kubernetes.ExecutorEvent_SHUTDOWN_REQ:
			log.Info("Stopping cke k8s wrapper")
			kw.KillProcesses()
			kw.callExecutor(execEvent.TaskId, kubernetes.WrapperEventResponse_KILLED, "OK")
			kw.stream.CloseSend()
		default:
			log.Warning("unknown executor message: " + execEvent.Type.String())
			break
		}
	}
	kw.stopHeartbeat()
	return nil
}

//调用os.MkdirAll递归创建文件夹
func createFile(filePath string) error {
	if !isExist(filePath) {
		err := os.MkdirAll(filePath, os.ModePerm)
		return err
	}
	return nil
}

// 判断所给路径文件/文件夹是否存在(返回true是存在)
func isExist(path string) bool {
	_, err := os.Stat(path) //os.Stat获取文件信息
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}

//将进程需要的文件保存到本地
func saveFiles(files map[string]*kubernetes.K8SFile) error {
	for _, file := range files {
		idx := strings.LastIndex(file.Path, "/")
		dirPath := file.Path[:idx]
		createFile(dirPath)

		err := ioutil.WriteFile(file.Path, file.Data, os.FileMode(file.Mode))
		if err != nil {
			return err
		}
	}
	return nil
}

func (kw *K8sWrapper) startProcess(taskID string, startInfo *kubernetes.ProcessInfo) {
	procInfo := kw.runnings[taskID]
	if procInfo != nil {
		kw.callExecutor(taskID, kubernetes.WrapperEventResponse_FAILED, "Process "+taskID+" already exists")
		return
	}

	err := saveFiles(startInfo.Files)
	if err != nil {
		log.Errorf("Save process %s file error: %s\n", taskID, err.Error())
		kw.callExecutor(taskID, kubernetes.WrapperEventResponse_FAILED, err.Error())
		return
	}

	process, err := createProcessRuntime(kw.nodeID, kw.socketPath, kw.outputPath, taskID, startInfo)
	if err != nil {
		log.Errorf("Create process %s error: %s\n", taskID, err.Error())
		kw.callExecutor(taskID, kubernetes.WrapperEventResponse_FAILED, err.Error())
		return
	}

	err = process.start()
	if err != nil {
		log.Errorf("Start process %s error: %s\n", taskID, err.Error())
		kw.callExecutor(taskID, kubernetes.WrapperEventResponse_FAILED, err.Error())
		return
	}

	kw.runnings[taskID] = process
	kw.callExecutor(taskID, kubernetes.WrapperEventResponse_RUNNING, "OK")
	log.Info("Started process " + taskID)

	go process.runWaitting(kw.taskStopped)
}

func (kw *K8sWrapper) taskStopped(taskID string, code kubernetes.WrapperEventResponseStatType, errMsg string) {
	kw.callExecutor(taskID, code, errMsg)
	delete(kw.runnings, taskID)
}

func (kw *K8sWrapper) stopProcess(taskID string) {
	process := kw.runnings[taskID]
	if process != nil {
		process.destory()
		log.Info("Stopped process " + taskID)
	} else {
		log.Warning("Can't found process " + taskID + " to stop")
	}
}

func (kw *K8sWrapper) message(taskID string) {
	//	kw.callExecutor(taskId, 0, "OK")
}

func (kw *K8sWrapper) callExecutor(taskID string, code kubernetes.WrapperEventResponseStatType, errMsg string) {
	kw.resetHeartbeat()

	// 向服务端发送 指令
	msg := &kubernetes.WrapperEvent{
		TaskId: taskID,
		Type:   kubernetes.WrapperEvent_RESPONSE,
		Resp: &kubernetes.WrapperEventResponse{
			Code:  code,
			Error: errMsg,
		},
	}

	if err := kw.stream.Send(msg); err != nil {
		log.Errorf("Send wrapper message error, task:%s %s", taskID, err.Error())
	} else {
		log.Infof("Send wrapper message ok, task:%s code:%d msg:%s", taskID, code, errMsg)
	}
}

//KillProcesses 停止一个进程
func (kw *K8sWrapper) KillProcesses() {
	processSlice := make([]*processRuntime, 0, len(kw.runnings))
	for _, process := range kw.runnings {
		processSlice = append(processSlice, process)
	}
	sort.Slice(processSlice, func(i, j int) bool {
		return processSlice[i].startIndex > processSlice[j].startIndex
	})

	//并发停止各进程
	var wg sync.WaitGroup
	for _, process := range processSlice {
		wg.Add(1)
		go func(p *processRuntime) {
			p.destory()
			wg.Done()
		}(process)
	}
	wg.Wait()
}

//CloseStream gRPC流
func (kw *K8sWrapper) CloseStream() error {
	if kw == nil || kw.stream == nil {
		return nil
	}
	return kw.stream.CloseSend()
}

//重置心跳时延
func (kw *K8sWrapper) resetHeartbeat() {
	kw.hbLock.Lock()
	defer kw.hbLock.Unlock()
	if kw.hbTimer != nil {
		kw.hbTimer.Stop()
		kw.hbTimer.Reset(time.Duration(kw.heartbeat) * time.Second)
	} else {
		kw.hbTimer = time.AfterFunc(time.Duration(kw.heartbeat)*time.Second, func() {
			kw.heartbeatLoop()
		})
	}
}

func (kw *K8sWrapper) stopHeartbeat() {
	kw.hbLock.Lock()
	defer kw.hbLock.Unlock()
	if kw.hbTimer != nil {
		kw.hbTimer.Stop()
	}
}

func (kw *K8sWrapper) heartbeatLoop() {
	//发送心跳
	msg := &kubernetes.WrapperEvent{
		Type:      kubernetes.WrapperEvent_HEARTBEAT,
		Heartbeat: kw.heartbeat,
	}
	if err := kw.stream.Send(msg); err != nil {
		log.Errorf("Send wrapper heartbeat error: %s", err.Error())
	} else {
		log.Info("Send wrapper heartbeat ok")
	}
	//重置心跳时延
	kw.resetHeartbeat()
}
