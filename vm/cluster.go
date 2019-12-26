package vm

import (
	"cke/cluster"
	"cke/log"
	"cke/vm/config"
	"cke/vm/model"
	"cke/vm/storage"
	"errors"
	"flag"
	"sync"

	"github.com/gogo/protobuf/proto"
)

/*****************
  Author: Chen
  Date: 2019/3/28
  Comp: ChinaUnicom
 ****************/

const CLUSTER_NAME = "cke-vm"

var (
	execPath = flag.String("vm_executor", "cke-vm-exec", "The process of mesos executor for CKE VM")
)

type Cluster struct {
	lock      sync.RWMutex
	name      string
	etcdCli   *storage.EtcdClient
	Tasks     map[string]*cluster.Task
	OverTasks map[string]*cluster.Task
	// StopTask  func(task *cluster.Task) (err error)
	cm cluster.Scheduler
}

func (c *Cluster) Initialize(sch cluster.Scheduler) error {
	log.Infof("Initializing a new cluster : %s", CLUSTER_NAME)
	c.name = CLUSTER_NAME
	c.cm = sch
	c.Tasks = make(map[string]*cluster.Task, 1)
	c.OverTasks = make(map[string]*cluster.Task, 1)
	return nil
}

func (c *Cluster) GetName() string {
	return c.name
}

//GetReservations 获得该集群需要预留的信息
func (c *Cluster) GetReservations() (reservations []*cluster.Reservation) {
	return nil
}

func (c *Cluster) GetInconsistentTasks() (bool, []*cluster.TaskInfo) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.Tasks == nil {
		return true, nil
	}

	var inConsistentTasks []*cluster.TaskInfo

	for _, task := range c.Tasks {
		// 如果task的期望状态和真实状态不一致
		if task.Status == nil {
			inConsistentTasks = append(inConsistentTasks, task.Info)
		}
	}
	return true, inConsistentTasks
}

//MarshalTask 将TaskInfo中的Body转换为proto的[]byte以便scheduler发送给mesos的executor
func (c *Cluster) MarshalTask(taskBody interface{}) ([]byte, error) {
	task, err := taskBody.(*model.LibVm)
	if err {
		taskBytes, err := proto.Marshal(task)
		if err != nil {
			return nil, err
		}
		return taskBytes, nil
	}
	return nil, errors.New("taskBody is not a VMTask")
}

func (c *Cluster) StopCluster() error {
	return nil
}

func (c *Cluster) RemoveCluster() (*sync.WaitGroup, error) {
	return nil, nil
}

func (c *Cluster) SetLeader(isNewFramework bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.Tasks = make(map[string]*cluster.Task)
	c.OverTasks = make(map[string]*cluster.Task)

	//c.etcdCli = etcdCli
	var err error
	c.etcdCli, err = storage.NewClient(&c.name)
	if err != nil {
		log.Error(err.Error())
	}

	if c.etcdCli != nil {
		c.etcdCli.GetTasks(func(taskName string, taskInfo *cluster.TaskInfo) {
			task := &cluster.Task{
				Info: taskInfo,
				Status: &cluster.TaskStatus{
					State: cluster.TASKMESOS_UNKNOWN,
				},
			}
			c.Tasks[taskName] = task
		})
	}
}

func (c *Cluster) GetTasks() (taskList []*cluster.Task) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	taskList = make([]*cluster.Task, 0, len(c.Tasks))
	for _, t := range c.Tasks {
		taskList = append(taskList, t)
	}
	return taskList
}

func (c *Cluster) GetTask(name string) (task *cluster.Task) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if c.Tasks[name] == nil {
		return
	}
	return c.Tasks[name]

}

func (c *Cluster) AddTask(taskInfo *cluster.TaskInfo) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	taskInfo.Executor = &cluster.ExecutorInfo{
		Cmd: *execPath,
		Res: &cluster.Resource{
			CPU: 0.1,
			Mem: 1,
		},
		Envs: map[string]string{
			"CEPH_MONITORS": config.CephMonitors,
		},
		/*Envs: map[string]string{
		    "MESOS_AGENT_ENDPOINT": "", // agent IP:Port
		    "MESOS_FRAMEWORK_ID": "",
		    "MESOS_EXECUTOR_ID": "",
		},*/
	}

	clsTask := &cluster.Task{
		Info: taskInfo,
	}
	if c.etcdCli != nil {
		err := c.etcdCli.SetTask(clsTask.Info.Name, taskInfo)
		if err != nil {
			return err
		}
	}

	if c.Tasks[taskInfo.Name] == nil {
		c.Tasks[taskInfo.Name] = clsTask
	} else {
		return errors.New("The task " + taskInfo.Name + " already exists.")
	}
	c.cm.ReviveOffer(c)
	return nil
}

func (c *Cluster) RemoveTask(taskName string) (taskInfo *cluster.TaskInfo, err error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	task := c.Tasks[taskName]
	if task != nil {
		taskInfo = task.Info
		*taskInfo.State = cluster.ExpectSTOP

		if c.etcdCli != nil {
			err := c.etcdCli.SetTask(taskName, taskInfo)
			if err != nil {
				return nil, err
			}
		}

		if task.Status != nil && (task.Status.State == cluster.TASKMESOS_FINISHED ||
			task.Status.State == cluster.TASKMESOS_KILLED ||
			task.Status.State == cluster.TASKMESOS_FAILED) {
			delete(c.Tasks, taskName)
			c.OverTasks[task.Status.Id] = task
			if c.etcdCli != nil {
				err = c.etcdCli.RemoveTask(taskName)
				if err != nil {
					return nil, err
				}
			}
		} else { //正常运行
			// TODO:
			err = c.cm.StopTask(c, task)
			if err != nil {
				return nil, err
			}
			// c.StopTask(task)
		}

		return task.Info, nil
	}
	err = errors.New("cannot find task:" + taskName)
	return nil, err
}

func (c *Cluster) UpdateTaskStatus(status *cluster.TaskStatus) {
	taskName := cluster.MesosTaksId2TaskName(status.Id)
	log.Infof("UpdateTaskStatus [%s]; state [%s]", taskName, status.State.String())
	delFlag := false
	c.lock.Lock()
	defer c.lock.Unlock()
	task := c.Tasks[taskName]
	if task != nil {
		task.Status = status
		if *task.Info.State == cluster.ExpectSTOP {
			if task.Status.State == cluster.TASKMESOS_FINISHED ||
				task.Status.State == cluster.TASKMESOS_KILLED ||
				task.Status.State == cluster.TASKMESOS_FAILED {
				delete(c.Tasks, taskName)
				c.OverTasks[status.Id] = task
				delFlag = true
			}
		}
	}
	var err error
	if c.etcdCli != nil && task != nil {
		if delFlag {
			c.etcdCli.RemoveTask(taskName)
			if err != nil {
				return
			}
		} else {
			err = c.etcdCli.SetTask(taskName, task.Info)
			if err != nil {
				return
			}
		}
	}
	return
}

func (c *Cluster) GetOverTasks() (taskList []*cluster.TaskInfo) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	taskList = make([]*cluster.TaskInfo, 0, len(c.OverTasks))
	for _, t := range c.OverTasks {
		taskList = append(taskList, t.Info)
	}
	return
}
