package executor

import (
	"cke/executor"
	"cke/kubernetes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strconv"
	"strings"
	"sync"

	"cke/log"
	"cke/utils"

	"github.com/containerd/cgroups"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"os"
)

const (
	EVN_EXEC_ENDPOINT = "CKE_K8S_EXEC_ENDPOINT"
	EVN_KUBE_NODE_ID  = "CKE_K8S_NODE_ID"
	EVN_OUTPUT_PATH   = "CKE_OUTPUT_PATH"
	ENV_HEARTBEAT     = "CKE_HEARTBEAT"
	EVN_HOST_CPUS     = "CKE_HOST_CPUS"
	EVN_HOST_MEM      = "CKE_HOST_MEM"
	EVN_NODE_CPUS     = "CKE_NODE_CPUS"
	EVN_NODE_MEM      = "CKE_NODE_MEM"
	ENV_PUBLISH_PORT  = "CKE_PUBLISH_PORT"

	WRAPPER_HEARTBEAT = "60" // executor 与 wrapper 的心跳时长(秒)

	kubelet_reserved_cpus = 0.1
	kubelet_reserved_mem  = 1024 * 1024 //KB
)

//KubeNodeContainer KubeNode Container
type KubeNodeContainer struct {
	shutdownWG     sync.WaitGroup //停止KubeNode容器用到的锁
	ctx            context.Context
	cli            *client.Client
	containerID    string
	tasks          map[string]*executor.TaskInfoWithResource
	stream         kubernetes.WrapperProto_EventCallServer
	caller         executor.Caller
	nodeID         string
	nodeName       string
	workPath       string
	cfs            bool
	cpus           float64
	mem            float64
	taskID         string //启动wrapper对应的task id
	nodeInfo       *kubernetes.K8SNodeInfo
	enableLXCFS    bool
	nodeDockerPath string
	connectedFunc  func(string) //wrapper连接后的回调函数
}

// create LXCFG mounts
func createLXCFSMount(lxcfsPath string) []mount.Mount {
	if !strings.HasSuffix(lxcfsPath, "/") {
		lxcfsPath += "/"
	}
	return []mount.Mount{
		{
			Type:     mount.TypeBind,
			Source:   lxcfsPath + "proc/cpuinfo",
			Target:   "/proc/cpuinfo",
			ReadOnly: false,
		},
		{
			Type:     mount.TypeBind,
			Source:   lxcfsPath + "proc/diskstats",
			Target:   "/proc/diskstats",
			ReadOnly: false,
		},
		{
			Type:     mount.TypeBind,
			Source:   lxcfsPath + "proc/meminfo",
			Target:   "/proc/meminfo",
			ReadOnly: false,
		},
		{
			Type:     mount.TypeBind,
			Source:   lxcfsPath + "proc/stat",
			Target:   "/proc/stat",
			ReadOnly: false,
		},
		{
			Type:     mount.TypeBind,
			Source:   lxcfsPath + "proc/swaps",
			Target:   "/proc/swaps",
			ReadOnly: false,
		},
		{
			Type:     mount.TypeBind,
			Source:   lxcfsPath + "proc/uptime",
			Target:   "/proc/uptime",
			ReadOnly: false,
		},
	}
}

//创建内部docker的工作目录，如果由命令行指定了则使用该配置，否则使用<当前的docker工作目录/cke>
func createDockerWorkPath(ctx context.Context, cli *client.Client, dockerPath string, nodeID string) (string, error) {
	var nodeDockerPath string
	//判断是否指定了胖容器内部docker的工作目录
	if len(dockerPath) <= 0 {
		//没有指定，获取当前的docker工作目录加上cke作为工作目录
		dockerInfo, err := cli.Info(ctx)
		if err != nil {
			return "", err
		}
		dockerPath = utils.AppendPath(dockerInfo.DockerRootDir, "cke")
	}
	//创建dockerd的工作目录在宿主上的对应目录
	nodeDockerPath = utils.AppendPath(dockerPath, nodeID)
	if _, err := utils.CreateDir(nodeDockerPath); err != nil {
		return "", err
	}
	log.Infof("Container %s work path: %s", nodeID, nodeDockerPath)
	return nodeDockerPath, nil
}

//CreateContainer 创建容器
func CreateContainer(
	taskID string,
	fwID string,
	nodeID string,
	image string,
	serverPath string,
	workPath string,
	nodeInfo *kubernetes.K8SNodeInfo,
	caller executor.Caller,
	dockerCfs bool,
	dockerPath string,
	lxcfsPath *string,
	cpus float64,
	mem float64) (*KubeNodeContainer, error) {

	const (
		WRAPPER_PATH    = "/usr/local/bin/cke-k8s-wrapper"
		EXECUTOR_SOCKET = "/executor.socket"
		TASK_WORK_PATH  = "/task_logs"
		DOCKER_WORK_DIR = "/var/lib/docker"
		CEPH_DEV_PATH   = "/dev"
	)

	//创建wrapper的输出目录
	wrapperOutputPath := utils.AppendPath(workPath, nodeID)
	if _, err := utils.CreateDir(wrapperOutputPath); err != nil {
		return nil, err
	}

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	//创建内部docker的工作目录
	nodeDockerPath, err := createDockerWorkPath(ctx, cli, dockerPath, fwID+"."+nodeID)
	reader, err := cli.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	io.Copy(os.Stdout, reader)

	containerConf := &container.Config{
		Hostname: nodeID,
		Image:    image,
		Env: []string{
			EVN_EXEC_ENDPOINT + "=" + EXECUTOR_SOCKET,
			EVN_KUBE_NODE_ID + "=" + nodeID,
			EVN_OUTPUT_PATH + "=" + TASK_WORK_PATH,
			ENV_HEARTBEAT + "=" + WRAPPER_HEARTBEAT,
			EVN_HOST_CPUS + "=" + strconv.FormatFloat(totalCpus, 'f', 2, 64),
			EVN_HOST_MEM + "=" + strconv.FormatInt(totalMem, 10),
		},
		Cmd: []string{WRAPPER_PATH},
		//Tty:   false,
	}

	hostConf := &container.HostConfig{
		Mounts: []mount.Mount{
			//与executor(wrapperServer)通信
			{
				Type:     mount.TypeBind,
				Source:   serverPath, //"/tmp/meos-cke/<executor_id>.socket",
				Target:   EXECUTOR_SOCKET,
				ReadOnly: false,
			},
			//容器中存放每个task的stdout,stderr的目录
			{
				Type:     mount.TypeBind,
				Source:   wrapperOutputPath, //executor的工作目录
				Target:   TASK_WORK_PATH,
				ReadOnly: false,
			},
			//容器中的dockerd使用的工作目录
			{
				Type:     mount.TypeBind,
				Source:   nodeDockerPath, //物理主机的dockerd工作目录+docker
				Target:   DOCKER_WORK_DIR,
				ReadOnly: false,
			},

			//用于挂载对物理机上的ceph客户端进行通信的通道。
			{
				Type:     mount.TypeBind,
				Source:   CEPH_DEV_PATH, //物理主机的dev块设备目录
				Target:   CEPH_DEV_PATH,
				ReadOnly: false,
			},
		},
		NetworkMode: "user",
		Privileged:  true,
	}

	//为每个节点添加 masters 的 hosts记录 name: kubernetes.default
	for _, record := range nodeInfo.DnsRecord {
		hostConf.ExtraHosts = append(hostConf.ExtraHosts,
			fmt.Sprintf("%s:%s", record.Host, record.Ip))
	}

	if len(*lxcfsPath) > 0 {
		hostConf.Mounts = append(hostConf.Mounts, createLXCFSMount(*lxcfsPath)...)
	}

	var networkingConfig *network.NetworkingConfig
	//加入边界节点对外暴露的服务端口
	if len(nodeInfo.PublishPorts) > 0 {
		hostConf.NetworkMode = "bridge"
		hostConf.PortBindings = make(map[nat.Port][]nat.PortBinding)
		containerConf.ExposedPorts = make(map[nat.Port]struct{})
		for _, p := range nodeInfo.PublishPorts {
			ports := strings.Split(p, ":")
			hostPort := nat.Port(ports[0])
			containerPort := nat.Port(ports[1])
			hostConf.PortBindings[containerPort] = []nat.PortBinding{
				nat.PortBinding{
					HostIP:   "",
					HostPort: hostPort.Port(),
				},
			}
			containerConf.ExposedPorts[containerPort] = struct{}{}
		}

		networkingConfig = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				"bridge": &network.EndpointSettings{
					IPAMConfig: &network.EndpointIPAMConfig{},
				},
			},
		}
	} else {
		networkingConfig = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				nodeInfo.Network: &network.EndpointSettings{
					IPAMConfig: &network.EndpointIPAMConfig{
						IPv4Address: nodeInfo.Ip,
					},
					IPAddress: nodeInfo.Ip,
				},
			},
		}
	}

	containerName := fmt.Sprintf("%s.%s.%08X", nodeID, fwID, rand.Int31())
	resp, err := cli.ContainerCreate(ctx, containerConf, hostConf, networkingConfig, containerName)
	if err != nil {
		return nil, err
	}

	if len(nodeInfo.PublishPorts) > 0 {
		//挂载管理网络
		endpointsConfig := &network.EndpointSettings{
			IPAMConfig: &network.EndpointIPAMConfig{
				IPv4Address: nodeInfo.Ip,
			},
			IPAddress: nodeInfo.Ip,
		}
		err = cli.NetworkConnect(ctx, nodeInfo.Network, resp.ID, endpointsConfig)
		if err != nil {
			return nil, err
		}
	}

	return &KubeNodeContainer{
		ctx:            ctx,
		cli:            cli,
		containerID:    resp.ID,
		tasks:          make(map[string]*executor.TaskInfoWithResource),
		caller:         caller,
		nodeID:         nodeID,
		nodeName:       containerName,
		workPath:       workPath,
		cfs:            dockerCfs,
		cpus:           cpus,
		mem:            mem,
		taskID:         taskID,
		nodeInfo:       nodeInfo,
		enableLXCFS:    len(*lxcfsPath) > 0,
		nodeDockerPath: nodeDockerPath,
	}, nil
}

//StartContainer 启动容器
func (kc *KubeNodeContainer) StartContainer(connectedFunc func(string)) error {
	//打开容器退出等待的锁
	kc.shutdownWG.Add(1)

	kc.connectedFunc = connectedFunc

	if err := kc.cli.ContainerStart(kc.ctx, kc.containerID, types.ContainerStartOptions{}); err != nil {
		kc.clearContainer() //删除Created状态的容器
		return err
	}
	return nil
}

func (kc *KubeNodeContainer) updateContainerResource(cpus float64, mem float64) error {
	cpusTotal := kc.cpus
	memTotal := kc.mem
	for _, task := range kc.tasks {
		cpusTotal += task.Cpus
		memTotal += task.Mem
	}

	cpusTotal += cpus
	memTotal += mem

	updateConfig := container.UpdateConfig{}
	if kc.cfs {
		updateConfig.CPUQuota = int64(cpusTotal * 100000)
	} else {
		// TODO 计算share值，依据
		agentCpus := 32.0
		updateConfig.CPUShares = int64(cpusTotal * 32768 / agentCpus)
	}
	updateConfig.RestartPolicy.Name = "no"
	updateConfig.RestartPolicy.MaximumRetryCount = 0
	updateConfig.Memory = int64(memTotal * 1024 * 1024)
	updateConfig.MemorySwap = updateConfig.Memory * 2
	_, err := kc.cli.ContainerUpdate(kc.ctx, kc.containerID, updateConfig)
	if err != nil {
		log.Errorf("Update container error: %s", err.Error())
	}
	return nil
}

func (kc *KubeNodeContainer) updateCGruopsResource(cpus float64, mem float64) error {
	cpusTotal := kc.cpus
	memTotal := int64(kc.mem)
	for _, task := range kc.tasks {
		cpusTotal += task.Cpus
		memTotal += int64(task.Mem)
	}

	cpusTotal += cpus
	memTotal += int64(mem)

	//MB转换为Byte
	memTotal = memTotal * 1024 * 1024

	resource := &specs.LinuxResources{
		CPU: &specs.LinuxCPU{},
		Memory: &specs.LinuxMemory{
			Limit: &memTotal,
		},
	}
	if kc.cfs {
		quota := int64(cpusTotal * 100000)
		resource.CPU.Quota = &quota
	} else {
		// TODO 计算share值，依据
		agentCpus := 32.0
		shares := uint64(cpusTotal * 32768 / agentCpus)
		resource.CPU.Shares = &shares
	}

	control, err := cgroups.Load(cgroups.V1, cgroups.StaticPath("docker/"+kc.containerID))
	if err != nil {
		return nil
	}
	return control.Update(resource)
}

func (kc *KubeNodeContainer) clearContainer() {
	//KubeNode内的容器
	log.Infof("Container %s work dir removing", kc.nodeID)
	if _, err := utils.RemoveDir(kc.nodeDockerPath); err != nil {
		log.Warningf("Remove KubeNode(%s) docker work_dir %s error: %s",
			kc.nodeID, kc.nodeDockerPath, err.Error())
	} else {
		//尝试删除父目录
		if _, err := utils.RemoveParentDir(kc.nodeDockerPath); err != nil {
			log.Warningf("Remove KubeNode(%s) docker work_dir %s error: %s",
				kc.nodeID, kc.nodeDockerPath, err.Error())
		} else {
			log.Infof("container %s work dir removd", kc.nodeID)
		}
	}

	//删除KubeNode本身
	conf := types.ContainerRemoveOptions{
		Force: true,
	}
	log.Infof("Container %s removing", kc.nodeID)
	if err := kc.cli.ContainerRemove(kc.ctx, kc.containerID, conf); err != nil {
		log.Warningf("Remove KubeNode(%s)%s error: %s",
			kc.nodeID, kc.containerID, err.Error())
	} else {
		log.Infof("Container %s removed", kc.nodeID)
	}

	if err := kc.cli.Close(); err != nil {
		log.Warningf("KubeNode(%s) container client close error: %s",
			kc.nodeID, err.Error())
	}
	log.Infof("Container %s closed", kc.nodeID)
}

//WaitingExited 等待容器退出
func (kc *KubeNodeContainer) WaitingExited() (int64, error) {
	statusCh, errCh := kc.cli.ContainerWait(
		kc.ctx, kc.containerID, container.WaitConditionNotRunning)
	defer func() {
		//结束时清除为容器准备的环境
		kc.clearContainer()
		kc.shutdownWG.Done()
	}()
	select {
	case err := <-errCh:
		if err != nil {
			return -1, err
		}
	case status := <-statusCh:
		var err error
		if status.Error != nil {
			err = errors.New(status.Error.Message)
		}
		return status.StatusCode, err
	}
	return -9999, nil
}

func (kc *KubeNodeContainer) initialize(
	stream kubernetes.WrapperProto_EventCallServer, ctx context.Context) error {
	// 传输初始化文件
	event := &kubernetes.ExecutorEvent{
		Type:   kubernetes.ExecutorEvent_SAVE_FILE,
		TaskId: kc.taskID,
		Files:  kc.nodeInfo.Files,
	}
	if err := stream.Send(event); err != nil {
		return err
	}
	var ret error
	for {
		select {
		case <-ctx.Done():
			ret = errors.New("wrapper has exited")
		default:
			// 接收从客户端发来的消息
			wrapperEvent, err := stream.Recv()
			if err == io.EOF {
				ret = errors.New("End of data stream sent by wrapper")
			}
			if err != nil {
				ret = errors.New("Error receiving data: " + err.Error())
			}
			if wrapperEvent.Type == kubernetes.WrapperEvent_RESPONSE {
				if wrapperEvent.Resp.Code == kubernetes.WrapperEventResponse_SAVED {
					log.Info("KubeNode " + kc.nodeID + " initialized.")
					return nil
				} else if wrapperEvent.Resp.Code == kubernetes.WrapperEventResponse_UNSAVED {
					ret = errors.New("KubeNode " + kc.nodeID + " save file error: " + wrapperEvent.Resp.Error)
				}
			} else {
				return errors.New("Wrong registration message")
			}
		}
		if ret != nil {
			kc.caller.CallTaskFailed(kc.taskID, ret.Error())
			break
		}
	}
	//TODO 未完成，需要对 k8s executor 的 startKubeNode 解除阻塞
	return ret
}

func (kc *KubeNodeContainer) eventLoop(stream kubernetes.WrapperProto_EventCallServer, ctx context.Context) error {
	kc.stream = stream

	kc.connectedFunc(kc.nodeID) //已经连接上了

	//ctx := kc.stream.Context()
	for {
		select {
		case <-ctx.Done():
			errMsg := "KubeNode(" + kc.nodeID + ") has exited"
			kc.failedAllTasks(errMsg)
			kc.caller.CallTaskFailed(kc.taskID, errMsg)
			log.Errorf(errMsg)
			return errors.New(errMsg)
		default:
			// 接收从客户端发来的消息
			wrapperEvent, err := kc.stream.Recv()
			if err == io.EOF {
				log.Info("End of data stream sent by wrapper")
				return nil
			}
			if err != nil {
				//通知master所有任务失败
				errMsg := "KubeNode(" + kc.nodeID + ") error: " + err.Error()
				kc.failedAllTasks(errMsg)
				kc.caller.CallTaskFailed(kc.taskID, errMsg)
				log.Errorf(errMsg)
				return errors.New(errMsg)
			}
			log.Debugf("Container event: wrapper:%s msg:%s", kc.nodeID, wrapperEvent)
			// 如果接收正常，则根据接收到的 字符串 执行相应的指令
			switch wrapperEvent.Type {
			case kubernetes.WrapperEvent_RESPONSE:
				if wrapperEvent.Resp.Code == kubernetes.WrapperEventResponse_FINISHED {
					err := kc.updateCGruopsResource(-kc.tasks[wrapperEvent.TaskId].Cpus, -kc.tasks[wrapperEvent.TaskId].Mem)
					if err != nil {
						log.Error("Update container resource error:(" + wrapperEvent.TaskId + ") " + err.Error())
					}
					delete(kc.tasks, wrapperEvent.TaskId)
					//告诉mesos master任务完成
					kc.caller.CallTaskFinished(wrapperEvent.TaskId)
				} else if wrapperEvent.Resp.Code == kubernetes.WrapperEventResponse_RUNNING {
					//告诉mesos master任务正在执行
					kc.caller.CallTaskRunning(wrapperEvent.TaskId)
				} else if wrapperEvent.Resp.Code == kubernetes.WrapperEventResponse_KILLED {
					if kc.tasks[wrapperEvent.TaskId] != nil { //如果是process中的task
						err := kc.updateCGruopsResource(-kc.tasks[wrapperEvent.TaskId].Cpus, -kc.tasks[wrapperEvent.TaskId].Mem)
						if err != nil {
							log.Error("Update container resource error:(" + wrapperEvent.TaskId + ") " + err.Error())
						}
						delete(kc.tasks, wrapperEvent.TaskId)
					}
					//告诉mesos master任务被杀死
					kc.caller.CallTaskKilled(wrapperEvent.TaskId)

				} else {
					err := kc.updateCGruopsResource(-kc.tasks[wrapperEvent.TaskId].Cpus, -kc.tasks[wrapperEvent.TaskId].Mem)
					if err != nil {
						log.Error("Update container resource error:(" + wrapperEvent.TaskId + ") " + err.Error())
					}
					delete(kc.tasks, wrapperEvent.TaskId)
					//告诉mesos master任务出错了
					kc.caller.CallTaskFailed(wrapperEvent.TaskId, wrapperEvent.Resp.Error)
				}
				break
			case kubernetes.WrapperEvent_MESSAGE:
				log.Debug("MessageType_MESSAGE:" + wrapperEvent.GetMsg())
				break
			case kubernetes.WrapperEvent_HEARTBEAT:
				break
			default:
				log.Warning("Unknown wrapper message: " + wrapperEvent.Type.String())
			}
		}
	}
	//return nil //不会被执行
}

func (kc *KubeNodeContainer) isWrapperTask(taskID string) bool {
	return kc.taskID == taskID
}

func (kc *KubeNodeContainer) hasTask(taskID string) bool {
	return kc.tasks[taskID] != nil
}

func (kc *KubeNodeContainer) failedAllTasks(errMsg string) {
	for taskID := range kc.tasks {
		kc.caller.CallTaskFailed(taskID, errMsg)
	}
	kc.tasks = make(map[string]*executor.TaskInfoWithResource)
}

// 为kubelet添加资源限制参数
//TODO 找到更好的设计模式描述如下业务
func appendArgs4KubeletTask(task *executor.TaskInfoWithResource) {
	k8sTask := task.Task.(*kubernetes.K8STask)
	if strings.HasSuffix(k8sTask.Process.Cmd, "kubelet") {
		//添加限制kubelet使用资源所需要的参数
		for i, arg := range k8sTask.Process.Args {
			if strings.HasPrefix(arg, "--kube-reserved=") {
				k8sTask.Process.Args = append(k8sTask.Process.Args[:i], k8sTask.Process.Args[i+1:]...)
				break
			}
		}
		k8sTask.Process.Args = append(k8sTask.Process.Args,
			fmt.Sprintf("--kube-reserved=cpu=%.2f,memory=%dKi", kubelet_reserved_cpus, kubelet_reserved_mem))

		systemReservedCPUs := totalCpus - task.Cpus - kubelet_reserved_cpus
		systemReservedMem := totalMem - (int64(task.Mem) * 1024) - kubelet_reserved_mem - 20000000
		log.Debug("##############", totalMem)
		for i, arg := range k8sTask.Process.Args {
			if strings.HasPrefix(arg, "--system-reserved=") {
				k8sTask.Process.Args = append(k8sTask.Process.Args[:i], k8sTask.Process.Args[i+1:]...)
				break
			}
		}
		k8sTask.Process.Args = append(k8sTask.Process.Args,
			fmt.Sprintf("--system-reserved=cpu=%.2f,memory=%dKi", systemReservedCPUs, systemReservedMem))
	}
}

//StartProcess 启动该容器内的一个进程
//启动前根据该进程对应task的资源，修改容器可使用的资源量
func (kc *KubeNodeContainer) StartProcess(taskID string, task *executor.TaskInfoWithResource) {
	if kc.tasks[taskID] == nil {
		kc.caller.CallTaskStarting(taskID)

		log.Infof("Call updateContainerResource %s CPU:%0.2f, Mem:%0.2f", taskID, task.Cpus, task.Mem)
		//调整container 的使用资源
		err := kc.updateCGruopsResource(task.Cpus, task.Mem)
		if err != nil {
			kc.caller.CallTaskFailed(taskID, "Update container resource error when launch task: "+taskID+". "+err.Error())
			return
		}

		k8sTask := task.Task.(*kubernetes.K8STask)
		if !kc.enableLXCFS {
			//添加kubelet需要的环境变量
			appendArgs4KubeletTask(task)
		}
		// 向服务端发送 指令
		event := &kubernetes.ExecutorEvent{
			Type:    kubernetes.ExecutorEvent_START_REQ,
			TaskId:  taskID,
			Process: k8sTask.Process,
		}
		kc.tasks[taskID] = task
		if err := kc.stream.Send(event); err != nil {
			log.Error("Error send executor event: " + err.Error())
			return
		}
		log.Info("send executor event: StartProcess")
	} else {
		kc.caller.CallTaskFailed(taskID, "Launch task "+taskID+" error: task has running")
	}
}

//StopProcess 停止容器中指定任务ID的对应进程
func (kc *KubeNodeContainer) StopProcess(taskID string) {
	if kc.tasks[taskID] != nil {
		event := &kubernetes.ExecutorEvent{
			Type:   kubernetes.ExecutorEvent_STOP_REQ,
			TaskId: taskID,
		}
		if err := kc.stream.Send(event); err != nil {
			log.Error("Error send executor event: " + err.Error())
			return
		}
		log.Info("send executor event: StopProcess")
	} else {
		//TODO task not find
		log.Info("task " + taskID + " has running")
	}

}

//DownloadData 导出容器中的指定文件，没有需求，未完成
func (kc *KubeNodeContainer) DownloadData() {
	//TODO download file
}

//Shutdown Shutdown容器，并清理现场
func (kc *KubeNodeContainer) Shutdown() {
	event := &kubernetes.ExecutorEvent{
		TaskId: kc.taskID,
		Type:   kubernetes.ExecutorEvent_SHUTDOWN_REQ,
	}
	log.Infof("Shutdown KubeNode(%s)", kc.nodeID)
	if err := kc.stream.Send(event); err != nil {
		log.Error("Error send executor event: " + err.Error())
		return
	}
	//等待KubeNode的退出
	kc.shutdownWG.Wait()

	//删除工作目录
	//TODO: 判断该代码是否多余，清除工作在func clearContainer()已经完成
	if _, err := utils.RemoveDir(kc.nodeDockerPath); err != nil {
		log.Warningf("Remove KubeNode's docker working directory (%s) error: %s", kc.nodeDockerPath, err.Error())
	}
}
