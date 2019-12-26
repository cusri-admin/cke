package cluster

import (
	clr "cke/cluster"
	"cke/log"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/coreos/etcd/pkg/types"
)

//FSMState 描述状态机的一种状态
type FSMState struct {
	Name  string
	Level int
}

//FSMEventType 事件类型
type FSMEventType string

//FSMEvent 状态机事件
type FSMEvent struct {
	Type    FSMEventType
	ProType ProcessType
}

//FSMHandler 处理该状态下的业务，并返回新的状态
type FSMHandler func(fsm *FSM, fsmEvent *FSMEvent) (*FSMState, []*clr.TaskInfo)

//FSM 有限状态机
type FSM struct {
	mu            sync.Mutex                             // 排他锁
	state         *FSMState                              // 当前状态
	handlers      map[string]map[FSMEventType]FSMHandler // 处理地图集，每一个状态都可以出发有限个事件，执行有限个处理
	publisher     *LogPublisher                          //状态机数据发布
	startingUp    bool                                   //是否启动
	childrens     []*FSM                                 //子状态机
	finishedState *FSMState                              //完成状态
	Cluster       *Cluster                               //fsm所属的cluster
	ReleatedNodes []*KubeNode                            //与本fsm相关联的节点，主要用于搜索启动任务
}

//FSMProxy 状态机代理
type FSMProxy struct {
	State         *FSMState   `json:"state"`
	StartingUp    bool        `json:"starting_up"`
	FinishedState *FSMState   `json:"finished_state"`
	Cluster       *Cluster    `json:"cluster"`
	ReleatedNodes []*KubeNode `json:"reated_nodes"`
}

// 获取当前状态
func (f *FSM) getState() *FSMState {
	return f.state
}

// GetFSMProxy : just for test.
func GetFSMProxy(fsm *FSM) *FSMProxy {
	return &FSMProxy{
		State:         fsm.state,
		StartingUp:    fsm.startingUp,
		FinishedState: fsm.finishedState,
		Cluster:       fsm.Cluster,
		ReleatedNodes: fsm.ReleatedNodes,
	}
}

// 设置当前状态
func (f *FSM) setState(newState *FSMState) {
	f.state = newState
}

//AddHandler 向状态机添加一个状态的事件处理方法
//参数：
//state *FSMState 指定的状态
//eventType FSMEventType 事件类型
//handler FSMHandler 对应的处理方法
func (f *FSM) AddHandler(state *FSMState, eventType FSMEventType, handler FSMHandler) *FSM {
	if _, ok := f.handlers[state.Name]; !ok {
		f.handlers[state.Name] = make(map[FSMEventType]FSMHandler)
	}
	if _, ok := f.handlers[state.Name][eventType]; ok {
		log.Warningf("状态(%s)事件(%s)已定义过", state.Name, eventType)
	}
	f.handlers[state.Name][eventType] = handler
	return f
}

//Call 事件处理
//参数：
//event *FSMEvent 一个具体的事件类型
//返回 在状态机当前状态下指定事件对应的任务，并决定是否进入下一个状态
func (f *FSM) Call(event *FSMEvent) (*FSMState, []*clr.TaskInfo) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.startingUp {
		log.Warning("FSM is stopped...")
		return f.state, make([]*clr.TaskInfo, 0)
	}
	events := f.handlers[f.getState().Name]
	if events == nil {
		return f.getState(), nil
	}
	var tasks []*clr.TaskInfo
	var fsmState *FSMState
	if handler, ok := events[event.Type]; ok {
		oldState := f.getState()
		fsmState, tasks = handler(f, event)
		f.setState(fsmState)
		newState := f.getState()
		if *newState != *oldState {
			//TODO: 需要与cluster.go中UpdateTaskStatus函数中的Log重构,用对象表示json
			msg := fmt.Sprintf("{\"type\":\"stat\",\"cluster\":\"%s\",\"node\":\"%s\",\"old_stat\":%d,\"old_stat_name\":\"%s\",\"new_stat\":%d,\"new_stat_name\":\"%s\",\"total_stat\":%d}",
				f.Cluster.ClusterName,
				f.ReleatedNodes[0].Name,
				(*oldState).Level,
				(*oldState).Name,
				(*newState).Level,
				(*newState).Name,
				f.finishedState.Level,
			)
			f.publisher.Log(msg)
			if f.ReleatedNodes[0].Type == KubeNodeType_Node { //当该状态机属于普通 node时,普通node的节点中包含了node自身和一个包含kubenodehc process的master
				log.Info("Cluster <", f.Cluster.ClusterName, "> Node <", f.ReleatedNodes[0].Name, "> from [", *oldState, "] to [", *newState, "]")
			} else if f.ReleatedNodes[0].Type == KubeNodeType_Console {
				log.Info("Cluster <", f.Cluster.ClusterName, "> Console <", f.ReleatedNodes[0].Name, "> from [", *oldState, "] to [", *newState, "]")
			} else {
				log.Info("Cluster <", f.Cluster.ClusterName, "> masters from [", *oldState, "] to [", *newState, "]")
			}

		}
	}
	return f.getState(), tasks
}

func (f *FSM) startFsm() {
	f.startingUp = true
}

func (f *FSM) stopFsm() {
	f.startingUp = false
}

func (f *FSM) addChild(child *FSM) {
	f.childrens = append(f.childrens, child)
}

func (f *FSM) deleteChild(child *FSM) {
	f.childrens = append(f.childrens, child)
	for i := 0; i < len(f.childrens); i++ {
		if f.childrens[i] == child {
			f.childrens = append(f.childrens[:i], f.childrens[i+1:]...)
			i--
		}
	}
}

//IsFinished 是否已经完成
func (f *FSM) IsFinished() bool {
	for _, chd := range f.childrens {
		if !chd.IsFinished() {
			return false
		}
	}
	return f.finishedState == f.state
}

//NewFSM 实例化FSM
func NewFSM(fsmType string, initState *FSMState, publisher *LogPublisher, cluster *Cluster, releatedNodes []*KubeNode) *FSM {
	k8sFsm := &FSM{
		state:         initState,
		handlers:      make(map[string]map[FSMEventType]FSMHandler),
		publisher:     publisher,
		startingUp:    true,
		childrens:     make([]*FSM, 0),
		Cluster:       cluster,
		ReleatedNodes: releatedNodes,
	}
	switch fsmType {
	case "master":
		k8sFsm.finishedState = &MasterFinishState
		initialMasterNextState()
		initialMasterHandler(k8sFsm)
		break
	case "node":
		k8sFsm.finishedState = &NodeFinishState
		initialNodeNextState()
		initialNodeHandler(k8sFsm)
	case "console":
		k8sFsm.finishedState = &ConsoleFinishState
		initialConsoleNextState()
		initialConsoleHandler(k8sFsm)
	}

	return k8sFsm
}

func initialMasterNextState() {
	NextState[&MasterWrapState] = &EtcdState
	NextState[&EtcdState] = &EtcdHcState
	NextState[&EtcdHcState] = &APIServerState
	NextState[&APIServerState] = &ManagerScheState
	NextState[&ManagerScheState] = &MasterDockerState
	NextState[&MasterDockerState] = &InitMasterCalicoState
	NextState[&InitMasterCalicoState] = &MasterHcState
	NextState[&MasterHcState] = &MasterFinishState
}

func initialNodeNextState() {
	NextState[&NodeWrapState] = &NodeConfState
	NextState[&NodeConfState] = &CRBuildingState
	NextState[&CRBuildingState] = &NodeDockerState
	NextState[&NodeDockerState] = &KubeletState
	NextState[&KubeletState] = &InitNodeCalicoState
	NextState[&InitNodeCalicoState] = &KubeHcState
	NextState[&KubeHcState] = &KubeProxyState
	NextState[&KubeProxyState] = &NodeFinishState
	NextState[&NodeFinishState] = &NodeFinishState
}

func initialConsoleNextState() {
	NextState[&ConsoleWrapState] = &KubectlConfState
	NextState[&KubectlConfState] = &CkeProxyState
	NextState[&CkeProxyState] = &GottyState
	NextState[&GottyState] = &ConsoleFinishState
	NextState[&ConsoleFinishState] = &ConsoleFinishState
}

func initialMasterHandler(k8sFsm *FSM) {
	k8sFsm.AddHandler(&MasterWrapState, ScheduleTaskType, ScheduTaskHandler)
	k8sFsm.AddHandler(&MasterWrapState, RollBackType, RollBackHandler)
	k8sFsm.AddHandler(&MasterWrapState, AddNodeType, AddNodeHandler)
	//k8sFsm.AddHandler(&MasterWrapState, RemoveNodeType, RemoveNodeHandler)

	k8sFsm.AddHandler(&EtcdState, ScheduleTaskType, ScheduTaskHandler)
	k8sFsm.AddHandler(&EtcdState, RollBackType, RollBackHandler)
	k8sFsm.AddHandler(&EtcdState, AddNodeType, AddNodeHandler)
	//k8sFsm.AddHandler(&EtcdState, RemoveNodeType, RemoveNodeHandler)

	k8sFsm.AddHandler(&EtcdHcState, ScheduleTaskType, ScheduTaskHandler)
	k8sFsm.AddHandler(&EtcdHcState, RollBackType, RollBackHandler)
	k8sFsm.AddHandler(&EtcdHcState, AddNodeType, AddNodeHandler)
	//k8sFsm.AddHandler(&EtcdHcState, RemoveNodeType, RemoveNodeHandler)

	k8sFsm.AddHandler(&APIServerState, ScheduleTaskType, ScheduTaskHandler)
	k8sFsm.AddHandler(&APIServerState, RollBackType, RollBackHandler)
	k8sFsm.AddHandler(&APIServerState, AddNodeType, AddNodeHandler)
	//k8sFsm.AddHandler(&ApiServerState, RemoveNodeType, RemoveNodeHandler)

	k8sFsm.AddHandler(&ManagerScheState, ScheduleTaskType, ScheduTaskHandler)
	k8sFsm.AddHandler(&ManagerScheState, RollBackType, RollBackHandler)
	k8sFsm.AddHandler(&ManagerScheState, AddNodeType, AddNodeHandler)
	//k8sFsm.AddHandler(&ManagerScheState, RemoveNodeType, RemoveNodeHandler)

	k8sFsm.AddHandler(&MasterDockerState, ScheduleTaskType, ScheduTaskHandler)
	k8sFsm.AddHandler(&MasterDockerState, RollBackType, RollBackHandler)
	k8sFsm.AddHandler(&MasterDockerState, AddNodeType, AddNodeHandler)
	//k8sFsm.AddHandler(&MasterDockerState, RemoveNodeType, RemoveNodeHandler)

	// k8sFsm.AddHandler(&InitCoreDnsState, ScheduleTaskType, ScheduTaskHandler)
	// k8sFsm.AddHandler(&InitCoreDnsState, RollBackType, RollBackHandler)
	// k8sFsm.AddHandler(&InitCoreDnsState, AddNodeType, AddNodeHandler)
	// k8sFsm.AddHandler(&InitCoreDnsState, RemoveNodeType, RemoveNodeHandler)

	//2019.07.25添加mater节点上创建Calico
	//======================================================================================
	k8sFsm.AddHandler(&InitMasterCalicoState, ScheduleTaskType, ScheduTaskHandler)
	k8sFsm.AddHandler(&InitMasterCalicoState, RollBackType, RollBackHandler)
	k8sFsm.AddHandler(&InitMasterCalicoState, AddNodeType, AddNodeHandler)
	k8sFsm.AddHandler(&InitMasterCalicoState, RemoveNodeType, RemoveNodeHandler)
	//======================================================================================

	k8sFsm.AddHandler(&MasterHcState, ScheduleTaskType, ScheduTaskHandler)
	k8sFsm.AddHandler(&MasterHcState, RollBackType, RollBackHandler)
	k8sFsm.AddHandler(&MasterHcState, AddNodeType, AddNodeHandler)
	k8sFsm.AddHandler(&MasterHcState, RemoveNodeType, RemoveNodeHandler)

	k8sFsm.AddHandler(&MasterFinishState, ScheduleTaskType, ScheduTaskHandler)
	k8sFsm.AddHandler(&MasterFinishState, RollBackType, RollBackHandler)
	k8sFsm.AddHandler(&MasterFinishState, AddNodeType, AddNodeHandler)
	k8sFsm.AddHandler(&MasterFinishState, RemoveNodeType, RemoveNodeHandler)

}

func initialNodeHandler(k8sFsm *FSM) {

	k8sFsm.AddHandler(&NodeWrapState, ScheduleTaskType, ScheduTaskHandler)
	k8sFsm.AddHandler(&NodeWrapState, RollBackType, RollBackHandler)
	k8sFsm.AddHandler(&NodeWrapState, AddNodeType, AddNodeHandler)
	k8sFsm.AddHandler(&NodeWrapState, RemoveNodeType, RemoveNodeHandler)

	k8sFsm.AddHandler(&NodeConfState, ScheduleTaskType, ScheduTaskHandler)
	k8sFsm.AddHandler(&NodeConfState, RollBackType, RollBackHandler)
	k8sFsm.AddHandler(&NodeConfState, AddNodeType, AddNodeHandler)
	k8sFsm.AddHandler(&NodeConfState, RemoveNodeType, RemoveNodeHandler)

	k8sFsm.AddHandler(&CRBuildingState, ScheduleTaskType, ScheduTaskHandler)
	k8sFsm.AddHandler(&CRBuildingState, RollBackType, RollBackHandler)
	k8sFsm.AddHandler(&CRBuildingState, AddNodeType, AddNodeHandler)
	k8sFsm.AddHandler(&CRBuildingState, RemoveNodeType, RemoveNodeHandler)

	k8sFsm.AddHandler(&NodeDockerState, ScheduleTaskType, ScheduTaskHandler)
	k8sFsm.AddHandler(&NodeDockerState, RollBackType, RollBackHandler)
	k8sFsm.AddHandler(&NodeDockerState, AddNodeType, AddNodeHandler)
	k8sFsm.AddHandler(&NodeDockerState, RemoveNodeType, RemoveNodeHandler)

	k8sFsm.AddHandler(&KubeletState, ScheduleTaskType, ScheduTaskHandler)
	k8sFsm.AddHandler(&KubeletState, RollBackType, RollBackHandler)
	k8sFsm.AddHandler(&KubeletState, AddNodeType, AddNodeHandler)
	k8sFsm.AddHandler(&KubeletState, RemoveNodeType, RemoveNodeHandler)

	k8sFsm.AddHandler(&KubeProxyState, ScheduleTaskType, ScheduTaskHandler)
	k8sFsm.AddHandler(&KubeProxyState, RollBackType, RollBackHandler)
	k8sFsm.AddHandler(&KubeProxyState, AddNodeType, AddNodeHandler)
	k8sFsm.AddHandler(&KubeProxyState, RemoveNodeType, RemoveNodeHandler)

	k8sFsm.AddHandler(&InitNodeCalicoState, ScheduleTaskType, ScheduTaskHandler)
	k8sFsm.AddHandler(&InitNodeCalicoState, RollBackType, RollBackHandler)
	k8sFsm.AddHandler(&InitNodeCalicoState, AddNodeType, AddNodeHandler)
	k8sFsm.AddHandler(&InitNodeCalicoState, RemoveNodeType, RemoveNodeHandler)

	k8sFsm.AddHandler(&KubeHcState, ScheduleTaskType, ScheduTaskHandler)
	k8sFsm.AddHandler(&KubeHcState, RollBackType, RollBackHandler)
	k8sFsm.AddHandler(&KubeHcState, AddNodeType, AddNodeHandler)
	k8sFsm.AddHandler(&KubeHcState, RemoveNodeType, RemoveNodeHandler)

	k8sFsm.AddHandler(&RemoveNodeState, ScheduleTaskType, rmvNdWithOtherHandler)
	k8sFsm.AddHandler(&RemoveNodeState, RollBackType, rmvNdWithOtherHandler)
	k8sFsm.AddHandler(&RemoveNodeState, AddNodeType, rmvNdWithOtherHandler)
	k8sFsm.AddHandler(&RemoveNodeState, RemoveNodeType, RemoveNodeHandler)

	k8sFsm.AddHandler(&NodeFinishState, ScheduleTaskType, ScheduTaskHandler)
	k8sFsm.AddHandler(&NodeFinishState, RollBackType, RollBackHandler)
	k8sFsm.AddHandler(&NodeFinishState, AddNodeType, AddNodeHandler)
	k8sFsm.AddHandler(&NodeFinishState, RemoveNodeType, RemoveNodeHandler)

}

func initialConsoleHandler(k8sFsm *FSM) {
	k8sFsm.AddHandler(&ConsoleWrapState, ScheduleTaskType, ScheduTaskHandler)
	k8sFsm.AddHandler(&ConsoleWrapState, RollBackType, RollBackHandler)
	k8sFsm.AddHandler(&ConsoleWrapState, AddNodeType, AddNodeHandler)
	k8sFsm.AddHandler(&ConsoleWrapState, RemoveNodeType, RemoveNodeHandler)

	k8sFsm.AddHandler(&KubectlConfState, ScheduleTaskType, ScheduTaskHandler)
	k8sFsm.AddHandler(&KubectlConfState, RollBackType, RollBackHandler)
	k8sFsm.AddHandler(&KubectlConfState, AddNodeType, AddNodeHandler)
	k8sFsm.AddHandler(&KubectlConfState, RemoveNodeType, RemoveNodeHandler)

	k8sFsm.AddHandler(&GottyState, ScheduleTaskType, ScheduTaskHandler)
	k8sFsm.AddHandler(&GottyState, RollBackType, RollBackHandler)
	k8sFsm.AddHandler(&GottyState, AddNodeType, AddNodeHandler)
	k8sFsm.AddHandler(&GottyState, RemoveNodeType, RemoveNodeHandler)

	k8sFsm.AddHandler(&CkeProxyState, ScheduleTaskType, ScheduTaskHandler)
	k8sFsm.AddHandler(&CkeProxyState, RollBackType, RollBackHandler)
	k8sFsm.AddHandler(&CkeProxyState, AddNodeType, AddNodeHandler)
	k8sFsm.AddHandler(&CkeProxyState, RemoveNodeType, RemoveNodeHandler)

	k8sFsm.AddHandler(&ConsoleFinishState, ScheduleTaskType, ScheduTaskHandler)
	k8sFsm.AddHandler(&ConsoleFinishState, RollBackType, RollBackHandler)
	k8sFsm.AddHandler(&ConsoleFinishState, AddNodeType, AddNodeHandler)
	k8sFsm.AddHandler(&ConsoleFinishState, RemoveNodeType, RemoveNodeHandler)

}

//所有状态及处理函数
var (
	MasterWrapState       = FSMState{"MasterWrapState", 0}       //start master container wrapper
	EtcdState             = FSMState{"EtcdState", 1}             //start etcd
	EtcdHcState           = FSMState{"EtcdHcState", 2}           //start etcd health check
	APIServerState        = FSMState{"ApiServer", 3}             //start ApiServer
	ManagerScheState      = FSMState{"ManagerScheState", 4}      //start controller manager and scheduler
	MasterDockerState     = FSMState{"MasterDockerState", 5}     //start docker daemon on kube master
	InitMasterCalicoState = FSMState{"InitMasterCalicoState", 6} //start InitMasterCalico
	MasterHcState         = FSMState{"MasterHcState", 7}         //start master health check
	MasterFinishState     = FSMState{"MasterFinishState", 8}     //状态机终态
	NodeWrapState         = FSMState{"NodeWrapState", 9}         //start node container wrapper
	NodeConfState         = FSMState{"NodeConfState", 10}        //start kube node configuration
	CRBuildingState       = FSMState{"CRBuildingState", 11}      //start bootstrap cluster role building (on master0)
	NodeDockerState       = FSMState{"NodeDockerState", 12}      //start docker daemon on kube node
	KubeletState          = FSMState{"KubeletState", 13}         //start Kubelet
	InitNodeCalicoState   = FSMState{"InitNodeCalicoState", 14}  //start InitCalico
	KubeHcState           = FSMState{"KubeHcState", 15}          //start Kubelet health check (on master0)
	KubeProxyState        = FSMState{"KubeProxyState", 16}       //start KubeProxy
	NodeFinishState       = FSMState{"NodeFinishState", 17}      //状态机终态
	ConsoleWrapState      = FSMState{"ConsoleWrapState", 18}     //start node container wrapper
	KubectlConfState      = FSMState{"KubectlConfState", 19}     //start kubectl configuration
	CkeProxyState         = FSMState{"CkeProxyState", 20}        //start cke proxy
	GottyState            = FSMState{"GottyState", 21}           //start Gotty
	ConsoleFinishState    = FSMState{"ConsoleFinishState", 22}   //状态机终态
	RemoveNodeState       = FSMState{"RemoveNodeState", -1}      // clear node from k8s and calico cluster

	//状态转移地图
	NextState = make(map[*FSMState]*FSMState)

	//事件类型
	ScheduleTaskType = FSMEventType("ScheduleTaskEvent")
	RollBackType     = FSMEventType("RollBackEvent")
	AddNodeType      = FSMEventType("AddNodeEvent")
	RemoveNodeType   = FSMEventType("RemoveNodeEvent")
	//收到offer后，规划启动任务
	ScheduTaskHandler = FSMHandler(func(fsm *FSM, fsmEvent *FSMEvent) (*FSMState, []*clr.TaskInfo) {
		fsmState := fsm.state //状态机当前状态
		taskList := make([]*clr.TaskInfo, 0)
		isComplete := true

		if fsmState == fsm.finishedState { //如果当前状态是完成状态，则尝试调用孩子状态机终的任务
			shuffle(fsm.childrens)
			for _, childFsm := range fsm.childrens {
				_, childTaskList := childFsm.Call(fsmEvent)
				taskList = append(taskList, childTaskList...)
			}
			return fsmState, taskList
		}
		processTypes := fsmStateToProcessType(fsmState)
		for _, kn := range fsm.ReleatedNodes {
			tasks, isCom := kn.GetInconsistentTasks(processTypes)
			if !isCom {
				taskList = append(taskList, tasks...)
				isComplete = false
			}
		}
		if isComplete {
			//如果本状态规划完成，切换到下个状态
			return NextState[fsmState], taskList
		}
		//如果本状态没有规划完成，继续启动，保持状态
		return fsmState, taskList

	})
	//任务失败回滚到指定状态，只会滚没有任务返回
	RollBackHandler = FSMHandler(func(fsm *FSM, fsmEvent *FSMEvent) (*FSMState, []*clr.TaskInfo) {
		fsmState := fsm.state //状态机当前状态
		rollBackState := fsmProcessTypeToState(fsmEvent.ProType)
		if rollBackState.Level < fsmState.Level {
			return rollBackState, nil
		}
		return fsmState, nil
	})

	//新增节点事件状态切换到CRBuildingState状态，只会滚没有任务返回
	AddNodeHandler = FSMHandler(func(fsm *FSM, fsmEvent *FSMEvent) (*FSMState, []*clr.TaskInfo) {
		fsmState := fsm.state //状态机当前状态
		//如果当前状态在CR状态之后，需要回滚，否则直接返回当前状态不切换。
		if fsmState.Level > CRBuildingState.Level {
			return &CRBuildingState, nil
		}
		return fsmState, nil
	})

	//删除节点事件状态，分情况进行转态转移
	RemoveNodeHandler = FSMHandler(func(fsm *FSM, fsmEvent *FSMEvent) (*FSMState, []*clr.TaskInfo) {
		fsmState := fsm.state //状态机当前状态
		if fsmState.Level != RemoveNodeState.Level && fsmState.Level <= MasterHcState.Level {
			//如果当前状态在masterhc任务及其之前业务, 则创建协程等待状态转到masterhc之后状态重新发出事件，进行节点清除工作
			go func() {
				for {
					if fsm.state.Level > MasterHcState.Level {
						fsm.Call(NewFsmEvent(&RemoveNodeType))
						break
					}
					//等待一段事件
					time.Sleep(2 * time.Second)
				}
			}()
			return fsmState, nil
		} else if fsmState.Level != RemoveNodeState.Level && fsmState.Level > MasterHcState.Level {
			//如果当前master node任务都已经正常启动，则直接转移状态进入节点清除工作, 并且指定其完成后下个状态回归到正常启动流程中
			NextState[&RemoveNodeState] = fsmState
			return &RemoveNodeState, nil
		} else {
			//如果当前是RemoveNodeState，则直接返回当前状态，不更改nextstate，防止连续删除节点请求导致removenode死循环
			return &RemoveNodeState, nil
		}
	})

	//当删除节点状态遇到非RemoveNode相关事件,则视情况调整 RemoveNode的next state
	rmvNdWithOtherHandler = FSMHandler(func(fsm *FSM, fsmEvent *FSMEvent) (*FSMState, []*clr.TaskInfo) {
		fsmState := fsm.state //状态机当前状态
		taskList := make([]*clr.TaskInfo, 0, 10)
		isCompleted := true
		switch fsmEvent.Type {
		case RollBackType: //如果正在进行节点痕迹清除，收到该事件，则判断是否更新remove state下个状态值
			rolToState := fsmProcessTypeToState(fsmEvent.ProType)
			if NextState[&RemoveNodeState].Level > rolToState.Level {
				NextState[&RemoveNodeState] = rolToState
			}
			return fsmState, nil
		case AddNodeType: //如果正在进行节点痕迹清除，收到该事件，则判断是否更新remove state下个状态值
			if NextState[&RemoveNodeState].Level > CRBuildingState.Level {
				NextState[&RemoveNodeState] = &CRBuildingState
			}
			return fsmState, nil
		case ScheduleTaskType: //如果正在进行节点痕迹清除，则获取相关任务进行清除
			processTypes := fsmStateToProcessType(&RemoveNodeState)
			for _, kn := range fsm.ReleatedNodes {
				tasks, isCom := kn.GetInconsistentTasks(processTypes)
				if !isCom {
					taskList = append(taskList, tasks...)
					isCompleted = false
				}
			}
			if isCompleted { //如果完成了remove node状态下的任务，则进行状态转移
				return NextState[&RemoveNodeState], nil
			}

		}
		return fsmState, taskList
	})
)

//NewFsmEvent 创建状态机的消息
func NewFsmEvent(eventType *FSMEventType, proType ...ProcessType) *FSMEvent {
	switch *eventType {
	case ScheduleTaskType:
		return &FSMEvent{Type: ScheduleTaskType}
	case RollBackType:
		return &FSMEvent{Type: RollBackType, ProType: proType[0]}
	case AddNodeType:
		return &FSMEvent{Type: AddNodeType}
	case RemoveNodeType:
		return &FSMEvent{Type: RemoveNodeType}
	default:
		return nil
	}
}

//FsmStateToProcessType 转换
func fsmStateToProcessType(state *FSMState) types.Set {
	processTypes := types.NewUnsafeSet()
	switch *state {
	case MasterWrapState:
		processTypes.Add(Process_MASTERWRAP.String())
		break
	case EtcdState:
		processTypes.Add(Process_ETCD.String())
		break
	case EtcdHcState:
		processTypes.Add(Process_ETCDHC.String())
		break
	case APIServerState:
		processTypes.Add(Process_APISERVER.String())
		break
	case ManagerScheState:
		processTypes.Add(Process_CONTROLLER.String())
		processTypes.Add(Process_SCHEDULER.String())
		break
	case MasterDockerState:
		processTypes.Add(Process_MASTERDOCKER.String())
		break
	case InitMasterCalicoState:
		processTypes.Add(Process_INITMASTERCALICO.String())
		break
	case MasterHcState:
		processTypes.Add(Process_MASTERHC.String())
		break
	case CRBuildingState:
		processTypes.Add(Process_CRBUILDING.String())
		break
	case NodeWrapState:
		processTypes.Add(Process_NODEWRAP.String())
		break
	case NodeConfState:
		processTypes.Add(Process_NODECONF.String())
		break
	case NodeDockerState:
		processTypes.Add(Process_NODEDOCKER.String())
		break
	case KubeletState:
		processTypes.Add(Process_KUBELET.String())
		break
	case InitNodeCalicoState:
		processTypes.Add(Process_INITNODECALICO.String())
		break
	case KubeHcState:
		processTypes.Add(Process_KUBELETHC.String())
		break
	case KubeProxyState:
		processTypes.Add(Process_KUBEPROXY.String())
		break
	case RemoveNodeState:
		processTypes.Add(Process_REMOVENODE.String())
		break
	case ConsoleWrapState:
		processTypes.Add(Process_CONSOLEWRAP.String())
		break
	case KubectlConfState:
		processTypes.Add(Process_KUBECTLCONF.String())
	case GottyState:
		processTypes.Add(Process_GOTTY.String())
	case CkeProxyState:
		processTypes.Add(Process_CKEPROXY.String())
		break
	}
	return processTypes
}

//FsmProcessTypeToState 转换
func fsmProcessTypeToState(proType ProcessType) *FSMState {
	switch proType {
	case Process_MASTERWRAP:
		return &MasterWrapState
	case Process_ETCD:
		return &EtcdState
	case Process_ETCDHC:
		return &EtcdHcState
	case Process_APISERVER:
		return &APIServerState
	case Process_CONTROLLER, Process_SCHEDULER:
		return &ManagerScheState
	case Process_MASTERDOCKER:
		return &MasterDockerState
	case Process_INITMASTERCALICO:
		return &InitMasterCalicoState
	case Process_MASTERHC:
		return &MasterHcState
	case Process_CRBUILDING:
		return &CRBuildingState
	case Process_NODEWRAP:
		return &NodeWrapState
	case Process_NODECONF:
		return &NodeConfState
	case Process_NODEDOCKER:
		return &NodeDockerState
	case Process_KUBELET:
		return &KubeletState
	case Process_INITNODECALICO:
		return &InitNodeCalicoState
	case Process_KUBELETHC:
		return &KubeHcState
	case Process_KUBEPROXY:
		return &KubeProxyState
	case Process_REMOVENODE:
		return &RemoveNodeState
	case Process_CONSOLEWRAP:
		return &ConsoleWrapState
	case Process_KUBECTLCONF:
		return &KubectlConfState
	case Process_GOTTY:
		return &GottyState
	case Process_CKEPROXY:
		return &CkeProxyState
	}

	return nil
}

func shuffle(a []*FSM) {
	for i := len(a) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		a[i], a[j] = a[j], a[i]
	}
}
