package cluster

import (
	"cke/log"
	"cke/storage"
	"errors"
	"reflect"
	"strings"
	"sync"
)

//MesosTask Cluster Manager和scheduler之间协作使用的
type MesosTask struct {
	ClusterName string
	Info        *TaskInfo
}

/*
* 根据集群名称创建ClusterInterface，其实现在各具体集群package中
 */
var (
	ClusterCreators = make(map[string]func(string, Scheduler) ClusterInterface)
	StorageType     string
	StorageServers  string
	FrameworkName   string
)

//ClusterManager 集群管理者结构体
type ClusterManager struct {
	lock        sync.RWMutex
	clusters    map[string]ClusterInterface
	device      ResDevice
	enableOffer bool
}

//CreateClusterManager 创建集群管理者
func CreateClusterManager() *ClusterManager {
	return &ClusterManager{
		clusters:    make(map[string]ClusterInterface),
		enableOffer: true,
	}
}

//Initialize 初始化集群管理器
func (cm *ClusterManager) Initialize(device ResDevice) {
	cm.device = device
}

//GetReservationOfClusters 获得每个集群需要预留的信息
func (cm *ClusterManager) GetReservationOfClusters() []*Reservation {
	cm.lock.RLock()
	defer cm.lock.RUnlock()
	reservations := make([]*Reservation, 0)
	for _, c := range cm.clusters {
		rs := c.GetReservations()
		if rs != nil {
			for _, r := range rs {
				reservations = append(reservations, r)
			}
		}
	}
	if len(reservations) > 0 {
		return reservations
	}
	return nil
}

//GetInconsistentTasksOfClusters 从集群管理者处获取所有集群待启动的任务，scheduler调用
func (cm *ClusterManager) GetInconsistentTasksOfClusters() (bool, map[ClusterInterface][]*TaskInfo) {
	// TODO 添加通过权重判断哪个集群具有使用Offer的优先级
	cm.lock.RLock()
	defer cm.lock.RUnlock()
	taskMap := make(map[ClusterInterface][]*TaskInfo)
	allFinished := true
	for _, c := range cm.clusters {
		finished, taskList := c.GetInconsistentTasks()
		if !finished {
			allFinished = false
		}
		if len(taskList) > 0 {
			taskMap[c] = taskList
		}
	}
	return allFinished, taskMap
}

/*
从mesos的taskid转换为集群名称
*/
func splitTaksID(mesosTaskID string) (string, string) {
	//mesos task名称规则“<clusterName>.<TaskName>.<random>”
	i := strings.Index(mesosTaskID, ".")
	j := strings.LastIndex(mesosTaskID, ".")
	if i < j && i > 0 {
		return mesosTaskID[0:i], mesosTaskID[i+1 : j]
	} else if i == j && i > 0 {
		return mesosTaskID[0:i], mesosTaskID[i+1:]
	} else if i == 0 {
		return "", mesosTaskID[1:]
	} else {
		return "", mesosTaskID
	}
}

//UpdateTaskStatus 更新集群任务状态
func (cm *ClusterManager) UpdateTaskStatus(taskStatus *TaskStatus) {
	//从mesos task ID中解析出cluster Name
	clustName, taskName := splitTaksID(taskStatus.Id)
	//更新对应cluster中的任务  vmClusters + clusters
	cm.lock.RLock()
	cluster := cm.clusters[clustName]
	cm.lock.RUnlock()
	if cluster != nil {
		taskStatus.TaskName = taskName
		cluster.UpdateTaskStatus(taskStatus)
	} else {
		log.Warningf("Update task status error: cluster \"%s\" can't be found.", clustName)
	}
}

//GetClustersByType 获取集群，用于REST api请求
func (cm *ClusterManager) GetClustersByType(clusterType reflect.Type) []ClusterInterface {
	clusters := make([]ClusterInterface, 0, len(cm.clusters))
	cm.lock.RLock()
	defer cm.lock.RUnlock()
	for _, cst := range cm.clusters {
		if clusterType == reflect.TypeOf(cst) {
			clusters = append(clusters, cst)
		}
	}
	return clusters
}

//GetCluster 获取集群，用于REST API请求
func (cm *ClusterManager) GetCluster(clusterName string) ClusterInterface {
	cm.lock.RLock()
	defer cm.lock.RUnlock()
	return cm.clusters[clusterName]
}

//AddCluster 添加集群，用于rest请求
func (cm *ClusterManager) AddCluster(c ClusterInterface) error {
	//1. 从rest请求的data转换为对应类型的cluster的struct
	//2. 初始化集群
	//3. 将集群加入cluster队列
	if err := c.Initialize(cm); err != nil {
		log.Errorf("Initialize cluster error: " + err.Error())
		return err
	}
	cm.lock.Lock()
	defer cm.lock.Unlock()
	if cm.clusters[c.GetName()] != nil {
		return errors.New("cluster " + c.GetName() + " has exited")
	}
	cm.clusters[c.GetName()] = c
	return nil
}

//RemoveCluster 移除集群，用于rest请求
func (cm *ClusterManager) RemoveCluster(name string) (ClusterInterface, error) {
	//1. 根据集群类型，从cluster 队列中取出集群对象
	//2. 向集群对象发送停止集群命令
	cm.lock.RLock()
	cluster := cm.clusters[name]
	cm.lock.RUnlock()
	if cluster == nil {
		return nil, errors.New("the cluster " + name + " does not exist!!!")
	}
	waitCluserDeleted, err := cluster.RemoveCluster()
	if err != nil {
		return nil, err
	}
	go func() {
		if waitCluserDeleted != nil {
			waitCluserDeleted.Wait()
		}
		cm.lock.Lock()
		defer cm.lock.Unlock()
		delete(cm.clusters, cluster.GetName())
	}()
	return cluster, nil
}

//SetLeader 设置本ClusterManager为Leader，负责写ETCD等storage
//当采用HA模式时：
//设置manager管理的集群leader信息，当集群启动时leader经过选举产生后，当本进程成为leader后通知本进程中所有cluster开始工作
func (cm *ClusterManager) SetLeader(storageCli storage.Storage, isNewFramework bool) {
	cm.lock.RLock()
	defer cm.lock.RUnlock()

	clusAndTypes, err := storageCli.GetClusterAndTypes()
	if err != nil {
		return
	}

	for name, clusType := range clusAndTypes {
		cm.clusters[name] = ClusterCreators[clusType](name, cm)
	}

	for _, c := range cm.clusters {
		c.SetLeader(isNewFramework)
	}
	//TODO reconcile 集群所有任务信息，更新到cluster

}

//StopTask 停止一个任务
func (cm *ClusterManager) StopTask(cluster ClusterInterface, task *Task) error {
	return cm.device.StopTask(task)
}

//Close 停止所有集群
func (cm *ClusterManager) Close() {
	cm.lock.RLock()
	defer cm.lock.RUnlock()
	for name, c := range cm.clusters {
		if _, err := c.RemoveCluster(); err != nil {
			log.Errorf("Remove cluster %s error: %s", name, err.Error())
		}
	}
}

//GetFrameworkID 获得framework的 id
func (cm *ClusterManager) GetFrameworkID() string {
	return cm.device.GetFrameworkID()
}

//GetShortFrameworkID 获得framework的 id 简写
func (cm *ClusterManager) GetShortFrameworkID() string {
	return cm.device.GetShortFrameworkID()
}

//RestoringOffer 恢复对该cluster的资源分配
func (cm *ClusterManager) ReviveOffer(cluster ClusterInterface) error {
	return cm.device.ReviveOffer()
}
