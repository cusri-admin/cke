package cluster

import (
	"encoding/json"
	"math"
	"strings"
	"time"
)

//TaskMesosState 描述Mesos中task的状态类型
type TaskMesosState int

const (
	TASKMESOS_STAGING  TaskMesosState = 1
	TASKMESOS_STARTING TaskMesosState = 2
	TASKMESOS_RUNNING  TaskMesosState = 3
	TASKMESOS_FINISHED TaskMesosState = 4
	TASKMESOS_KILLING  TaskMesosState = 5
	TASKMESOS_KILLED   TaskMesosState = 6
	TASKMESOS_FAILED   TaskMesosState = 7
	TASKMESOS_LOST     TaskMesosState = 8
	TASKMESOS_UNKNOWN  TaskMesosState = 9
)

func (s TaskMesosState) Int() int {
	return int(s)
}

func (s TaskMesosState) String() string {
	switch s {
	case TASKMESOS_STAGING:
		return "STAGING"
	case TASKMESOS_STARTING:
		return "STARTING"
	case TASKMESOS_RUNNING:
		return "RUNNING"
	case TASKMESOS_FINISHED:
		return "FINISHED"
	case TASKMESOS_KILLING:
		return "KILLING"
	case TASKMESOS_KILLED:
		return "KILLED"
	case TASKMESOS_FAILED:
		return "FAILED"
	case TASKMESOS_LOST:
		return "LOST"
	case TASKMESOS_UNKNOWN:
		return "UNKNOWN"
	default:
		return "UNKNOWN"
	}
}

//MarshalJSON 将TaskMesosState编码为字符串
func (s TaskMesosState) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

//UnmarshalJSON 从字符串解码TaskMesosState
func (s *TaskMesosState) UnmarshalJSON(b []byte) error {
	var state string
	json.Unmarshal(b, &state)
	switch strings.ToUpper(state) {
	case TASKMESOS_STAGING.String():
		*s = TASKMESOS_STAGING
		break
	case TASKMESOS_STARTING.String():
		*s = TASKMESOS_STARTING
		break
	case TASKMESOS_RUNNING.String():
		*s = TASKMESOS_RUNNING
		break
	case TASKMESOS_FINISHED.String():
		*s = TASKMESOS_FINISHED
		break
	case TASKMESOS_KILLING.String():
		*s = TASKMESOS_KILLING
		break
	case TASKMESOS_KILLED.String():
		*s = TASKMESOS_KILLED
		break
	case TASKMESOS_FAILED.String():
		*s = TASKMESOS_FAILED
		break
	case TASKMESOS_LOST.String():
		*s = TASKMESOS_LOST
		break
	case TASKMESOS_UNKNOWN.String():
		*s = TASKMESOS_UNKNOWN
		break
	}
	return nil
}

//ExpectStatus cluster集群中各种对象的目标运行状态
type ExpectStatus int

//描述集群中各种对象的目标运行状态
const (
	ExpectRUN      ExpectStatus = 3
	ExpectFINISHED ExpectStatus = 4
	ExpectSTOP     ExpectStatus = math.MaxInt64
)

func (s ExpectStatus) String() string {
	switch s {
	case ExpectRUN:
		return "RUN"
	case ExpectFINISHED:
		return "FINISHED"
	case ExpectSTOP:
		return "STOP"
	default:
		return "UNKNOWN"
	}
}

//MarshalJSON将ExpectStatus编码为字符串
func (s ExpectStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

//UnmarshalJSON从字符串解码ExpectStatus
func (s *ExpectStatus) UnmarshalJSON(b []byte) error {
	var state string
	json.Unmarshal(b, &state)
	switch strings.ToUpper(state) {
	case ExpectRUN.String():
		*s = ExpectRUN
		break
	case ExpectFINISHED.String():
		*s = ExpectFINISHED
		break
	case ExpectSTOP.String():
		*s = ExpectSTOP
		break
	}
	return nil
}

func (s ExpectStatus) Int() int {
	return int(s)
}

//Reservation 资源预留
type Reservation struct {
	AgentID         string                   `json:"agent_id"`            //预留的主机ID
	Host            *string                  `json:"host"`            //希望预留的主机
	ReservationID   string                   `json:"reservation_id"`            //预留资源名称
	Target          *Resource                `json:"target"`            //目标资源预留量
	Current         *Resource                `json:"current"`            //当前预留资源量
	GetUsedResource func() (*Resource, bool) `json:"-"`            //获得已使用资源量
	Callback        func()                   `json:"-"`            //取消预留后的回调
	IsNeedOperation bool                     `json:"is_need_operation"`            //是否需要操作的标识
	PreReserved     bool                     `json:"pre_reserved"` //预留资源是否已经在系统外完成
}

//TaskInfo 任务信息
type TaskInfo struct {
	//任务名，对应的mesos task的TaskId为该字段加上"."再加上一个16进制表示的64bit的随机数
	//任务名在framework中不能重复，唯一表示一个进程或虚拟机
	Name string `json:"name"`

	//任务类型。当前类型有两种：K8s,VM
	//随着任务类型的不同，Info字段承载着不同(k8s/vm)的数据，
	Type string `json:"type"`
	//希望运行该任务的主机
	Host *string `json:"host"`
	//根据使用的不同，进一步指定的任务数据结构
	Body interface{} `json:"-"`

	//使用的各种资源
	Res *Resource `json:"res"`

	//启动该Task的Executor的信息
	Executor *ExecutorInfo `json:"-"`

	//描述这个task的希望状态
	State *ExpectStatus `json:"state"`

	Reservation *Reservation//该任务希望运行在哪一个预留资源上
}

//TaskStatus 描述一个任务(k8s进程/VM)在mesos执行的情况
type TaskStatus struct {
	//一个任务所对应的mesos task的TaskID
	Id string `json:"id"`
	//task Name
	TaskName string `json:"task_name"`
	//mesos AgentID
	AgentId string `json:"agent_id"`
	//任务更新时的信息
	Message *string `json:"message"`
	//一个任务在mesos上对应的运行情况
	State TaskMesosState `json:"state"`
	//任务状态的更新时间
	StatusTime time.Time `json:"status_time"`
}

//Task 任务在内存中的存储结构
type Task struct {
	Info   *TaskInfo   `json:"info"`
	Status *TaskStatus `json:"status"`
}

//UnmarshalTask 由于是自定义executor，在mesos内部利用mesos task传递私有task时的数据编解码函数
func UnmarshalTask(data []byte) (*TaskInfo, error) {
	var t TaskInfo
	err := json.Unmarshal(data, &t)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func MarshalTask(task *TaskInfo) (data []byte, err error) {
	data, err = json.Marshal(task)
	return
}

func MesosTaksId2TaskName(mesosTaskId string) string {
	i := strings.Index(mesosTaskId, ".")
	j := strings.LastIndex(mesosTaskId, ".")
	if i < j && j > 0 {
		return mesosTaskId[i+1 : j]
	} else {
		return mesosTaskId
	}
}
