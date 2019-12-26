package cluster

import (
	cc "cke/cluster"
	mesos "cke/framework"
	"cke/kubernetes"
	"cke/utils"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/etcd/pkg/types"
)

//Process K8s集群中“进程”的结构体
type Process struct {
	Name       string          `json:"name"`
	Type       ProcessType     `json:"type"`
	Res        *cc.Resource    `json:"res"`
	Envs       []string        `json:"envs"`
	Options    []string        `json:"options"`
	Cmd        string          `json:"cmd"`
	ConfigJSON interface{}     `json:"config_file,omitempty"` //process 中json格式的配置文件，如kubelet中的config参数
	Task       *cc.Task        `json:"task"`
	Attributes []*Attribute    `json:"attributes"`
	Expect     cc.ExpectStatus `json:"expect"`
	node        *KubeNode
	StartTime   time.Time
	listenChans map[ListenType]([]chan ListenType) //用于监听process中task的状态变化
}

//Initialize 初始化进程
func (p *Process) Initialize(node *KubeNode) error {
	p.node = node
	p.StartTime = time.Now()
	//p.Expect = cc.ExpectRUN

	cfg := node.Conf.GetProcessConf(p.Type.String())
	if cfg == nil {
		return errors.New("Process " + p.Type.String() + " config can't be found.")
	}
	//校验数据正确性
	resConf := cfg.GetRes()
	if resConf != nil {
		cpu, err := resConf.GetCPU()
		if err != nil {
			return err
		}
		mem, err := resConf.GetMem()
		if err != nil {
			return err
		}
		p.Res = &cc.Resource{
			CPU: cpu,
			Mem: mem,
		}
	}

	//校验数据正确性
	return p.checkFormat()
}

func (p *Process) mayRollBack(status *cc.TaskStatus, cluster *Cluster, node *KubeNode) {
	if p.isNeedSchedule() {
		node.fsm.Call(NewFsmEvent(&RollBackType, p.Type))
	}
}

func (p *Process) notifyListener() {
	if p.listenChans == nil {
		return
	}
	//如果出现”停止“事件则通知相应chan
	if p.isStopped() {
		for _, ch := range p.listenChans[Listen_Stopped] {
			ch <- Listen_Stopped
		}
		p.listenChans[Listen_Stopped] = make([]chan ListenType, 0)
	}

	//如果出现”运行“事件则通知相应chan
	if p.isStarted() {
		for _, ch := range p.listenChans[Listen_Started] {
			ch <- Listen_Started
		}
		p.listenChans[Listen_Started] = make([]chan ListenType, 0)
	}
}

//RegistListenner 注册一个消息通道
func (p *Process) RegistListener(listenType ListenType) <-chan ListenType {
	if p.listenChans == nil {
		p.listenChans = make(map[ListenType][]chan ListenType)
	}

	if p.listenChans[listenType] == nil {
		p.listenChans[listenType] = make([]chan ListenType, 0)
	}
	listenChan := make(chan ListenType)
	p.listenChans[listenType] = append(p.listenChans[listenType], listenChan)

	return listenChan

}

func (p *Process) clearStatus() {
	if p.Task != nil {
		p.Task.Status = nil
	}
}

//UpdateTaskStatus 更新任务状态
func (p *Process) UpdateTaskStatus(status *cc.TaskStatus, node *KubeNode, cluster *Cluster) bool {
	if p.Task.Info.Name == status.TaskName {
		p.Task.Status = status
		//对临时任务的运行时状态需要更新到etcd中，防止leader切换时重新启动
		if status.State == cc.TASKMESOS_FINISHED {
			cluster.storageCli.StoreTask(node, p, p.Task)
		}
		if status.State == cc.TASKMESOS_RUNNING {
			p.StartTime = time.Now()
		}
		p.mayRollBack(status, cluster, node)
		p.notifyListener()
		if p.Expect == cc.ExpectSTOP {
			if status.State != cc.TASKMESOS_RUNNING {
				//TODO:删除自己从KubeNode
			}
		}
		// TODO 缺少更新etcd的代码
		return true
	}
	return false
}

//StopTask 停止任务
func (p *Process) StopTask(cluster *Cluster) {
	p.Expect = cc.ExpectSTOP
	//判断该进程对应的任务是否需要发送停止命令
	if p.Task != nil && p.Task.Status != nil {
		if p.Task.Status.State == cc.TASKMESOS_STAGING ||
			p.Task.Status.State == cc.TASKMESOS_STARTING ||
			p.Task.Status.State == cc.TASKMESOS_RUNNING {
			cluster.sch.StopTask(cluster, p.Task)
		}
	}
}

//生成该process的Task对象
func (p *Process) createTask(clusterName string, kubeNode *KubeNode, k8sTask *kubernetes.K8STask) error {
	p.Expect = p.Type.getTargetStatus()
	taskInfo := &cc.TaskInfo{
		Name:        kubeNode.Name + "." + p.Name,
		Type:        "k8s",
		Host:        &kubeNode.RealHost,
		Body:        k8sTask,
		Res:         p.Res,
		Executor:    getExecutorInfo(clusterName, kubeNode.Name),
		State:       &p.Expect,
		Reservation: kubeNode.Reservation,
	}
	p.Task = &cc.Task{
		Info: taskInfo,
	}
	return nil
}

//createProcessTask 使用配置及用户的定义生成进程对应的任务
func (p *Process) createProcessTask(cluster *Cluster, kubeNode *KubeNode, args ...interface{}) (*kubernetes.K8STask, error) {
	cfg := kubeNode.Conf.GetProcessConf(p.Type.String())
	if cfg == nil {
		return nil, errors.New("Process " + p.Type.String() + " config can't be found.")
	}

	//使用用户自定义的命令参数更新配置，如果可以的话
	for idx, arg := range p.Options {
		if strings.HasPrefix(arg, "--") {
			if (idx+1) < len(p.Options) && !strings.HasPrefix(p.Options[idx+1], "--") {
				//如果arg的flag后边有值，则放入对应的值
				cfg.SetArg(arg, p.Options[idx+1])
			} else {
				//如果arg的flag后边没有值，则置为特殊字符
				cfg.SetArg(arg, "")
			}
		}
	}

	//使用用户自定义的环境变量更新配置，如果可以的话
	for _, env := range p.Envs {
		envs := strings.Split(env, "=")
		if len(envs) >= 2 {
			cfg.SetEnv(envs[0], envs[1])
		} else if len(envs) == 1 {
			cfg.SetEnv(envs[0], "")
		}
	}

	//添加通用环境变量
	registry := getRegistryDNS()
	if registry != nil {
		cfg.SetEnv("DOCKER_REGISTRY", registry.Host)
	}

	//根据每个process的定制化函数，定制化这个task
	customizeFunc := p.Type.getCustomizeTaskFunc()
	if customizeFunc != nil {
		if err := customizeFunc(cfg, p, kubeNode, cluster, args...); err != nil {
			return nil, err
		}
	}

	//开始生成具体的K8STask
	k8sTask := &kubernetes.K8STask{
		NodeId:   kubeNode.NodeID,
		TaskType: kubernetes.K8STaskType_START_PROCESS,
		Image:    cfg.GetImagePath(),
		Process: &kubernetes.ProcessInfo{
			Cmd: cfg.Cmd,
		},
	}
	//参数
	for _, para := range cfg.Args {
		k8sTask.Process.Args = append(k8sTask.Process.Args, para.Key)
		if len(para.Value) > 0 {
			k8sTask.Process.Args = append(k8sTask.Process.Args, para.Value)
		}
	}
	//环境变量
	for _, para := range cfg.Envs {
		k8sTask.Process.Envs = append(k8sTask.Process.Envs, fmt.Sprintf("%s=%s", para.Key, para.Value))
	}
	//关联文件
	k8sTask.Process.Files = cfg.Files

	return k8sTask, nil
}

func (p *Process) generateK8sTask(cluster *Cluster, kubeNode *KubeNode, args ...interface{}) error {
	k8sTask, err := p.createProcessTask(cluster, kubeNode, args...)
	if err != nil {
		return err
	}

	return p.createTask(cluster.ClusterName, kubeNode, k8sTask)
}

func (p *Process) isNeedSchedule() bool {
	if p.Task == nil {
		return false
	}
	if p.Task.Status == nil { //unKnown 状态
		return true
	}
	if p.Task.Status.State.Int() > p.Task.Info.State.Int() {
		return true
	}
	return false
}

func (p *Process) isStarted() bool {
	if p.Task != nil && p.Task.Status != nil {
		if p.Task.Info.State.Int() == p.Task.Status.State.Int() {
			return true
		}
	}
	return false
}

//判断process是否被停止成功
func (p *Process) isStopped() bool {
	if p.Expect == cc.ExpectSTOP && p.Task.Status.State != cc.TASKMESOS_RUNNING {
		return true
	}
	return false
}

//GetInconsistentTasks 获得当前集群需要启动的任务
func (p *Process) GetInconsistentTasks(processTypes types.Set) (*cc.TaskInfo, bool) {
	if processTypes.Contains(p.Type.String()) {
		if p.isStarted() {
			return nil, true
		}
		if p.isNeedSchedule() {
			return p.Task.Info, false
		}
		return nil, false
	}
	return nil, true
}

//读取该进程的stderr和/或stdout文件
func (p *Process) getStdFile(nodeID string, fileName string,
	offset uint64, length uint64) ([]byte, error) {
	if p.Task.Status != nil {
		filePath := "/" + p.Task.Info.Executor.ID + "/runs/latest/" + nodeID + "/" + p.Task.Status.Id + "/" + fileName
		return mesos.AgentManager.ReadAgentFile(p.Task.Status.AgentId, filePath, offset, length)
	}
	return nil, utils.NewHttpError(http.StatusBadGateway, "Process "+p.Name+" is not running in "+nodeID)
}

//CheckFormat 检查Process数据的格式错误
func (p *Process) checkFormat() error {
	if p.Res == nil {
		return errors.New("Process " + p.Name + " has no resources")
	}
	return nil
}

//删除从storage中获取到的老的集群任务，包括 finished 的任务及 长运行任务状态的状态
func (p *Process) trimOldInfo() error {
	p.Task = nil
	return nil
}

//PrintProcessDebug 打印节点的debug信息
func (p *Process) PrintProcessDebug() {
	fmt.Printf("    Process: %s, type: %s\n", p.Name, p.Type)
	task, ok := p.Task.Info.Body.(*kubernetes.K8STask)
	if !ok {
		fmt.Printf("      task: error: task is not a K8STask\n")
	}
	fmt.Printf("      cmd: %s\n", task.Process.Cmd)
	fmt.Printf("      Args:\n")
	for _, v := range task.Process.Args {
		fmt.Printf("        \"%s\"\n", v)
	}
	fmt.Printf("      Envs:\n")
	for _, v := range task.Process.Envs {
		fmt.Printf("        \"%s\"\n", v)
	}
}
