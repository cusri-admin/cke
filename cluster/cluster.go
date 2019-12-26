package cluster

import "sync"

//ResDevice 资源驱动层应该具备的行为
type ResDevice interface {
	//StopTask 停止一个任务
	StopTask(task *Task) (err error)

	//GetFrameworkID 获得framework的id
	GetFrameworkID() string

	//GetShortFrameworkID 获得framework的短id
	GetShortFrameworkID() string

	//恢复对该cluster的资源分配
	ReviveOffer() error
}

//Scheduler 应该具备的行为
type Scheduler interface {
	//StopTask 停止一个任务
	StopTask(cluster ClusterInterface, task *Task) (err error)

	//GetShortFrameworkID 获得framework的FrameworkID
	GetShortFrameworkID() string

	//恢复对该cluster的资源分配
	ReviveOffer(cluster ClusterInterface) error
}

//ClusterInterface 在CKE中每个集群需要实现的接口
//实现该接口的集群统一被Cluster Manager管理
type ClusterInterface interface {
	//初始化集群
	Initialize(sch Scheduler) error

	//更新集群内任务状态
	UpdateTaskStatus(status *TaskStatus)

	//获得集群的资源预留的信息
	GetReservations() (reservations []*Reservation)

	//将TaskInfo中的Body转换为[]byte以便scheduler发送给mesos
	MarshalTask(taskBody interface{}) ([]byte, error)

	//获得需要执行的任务列表
	GetInconsistentTasks() (finished bool, taskList []*TaskInfo)

	//得到集群名称
	GetName() string

	//RemoveCluster 将异步删除集群(返回的sync.WaitGroup，阻塞到删除完成)
	RemoveCluster() (*sync.WaitGroup, error)

	//设置该集群在HA模式中为Leader
	SetLeader(isNewFramework bool)
}
