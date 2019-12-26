package exec

import (
	"cke/executor"
	"cke/log"
	"cke/vm/driver"
	"cke/vm/model"
	"fmt"
	"github.com/gogo/protobuf/proto"
	"github.com/libvirt/libvirt-go"
)

type Executor struct {
	caller  executor.Caller
	driver  *driver.LibVirtDriver
	context *driver.LibVirtContext
}

func NewExecutor() (*Executor, error) {
	//1. 初始化LibvirtDriver
	d, err := driver.NewLibVirtDriver()
	if err != nil {
		return nil, fmt.Errorf("Initialize libvirt driver(qemu connection) error: %s", err)
	}
	context := d.InitLibVirtContext()
	log.Infof("cke-kvm executor has been initialized....")
	return &Executor{
		driver:  d,
		context: context,
	}, nil
}

func (exec *Executor) UnmarshalTaskBody(taskBody []byte) (interface{}, error) {
	task := &model.LibVm{}
	err := proto.Unmarshal(taskBody, task)
	if err != nil {
		return nil, err
	}
	return task, nil
}

func (exec *Executor) Started() {
	go exec.ListenOnVmState()
	log.Info("Exectutor started...")
}

func (exec *Executor) Registered(caller executor.Caller) {
	exec.caller = caller
}

func (exec *Executor) Launch(taskId string, task *executor.TaskInfoWithResource) error {
	log.Infof("Starting to Launch Task: %s", taskId)

	//TODO: taskId与domain做关联?
	vm := task.Task.(*model.LibVm)

	go func() {
		err := exec.context.CreateDomain(vm, taskId)
		if err != nil {
			log.Errorf("Ops.... Error: %s", err)
			//return executor.TASKRESULT_FAILED, err
			exec.caller.CallTaskFailed(taskId, fmt.Sprint(err))
		}
		//TODO 是否提供重新创建机制？在事件中处理？
	}()
	exec.caller.CallTaskStarting(taskId)
	return nil
}

func (exec *Executor) Kill(taskId string) error {
	log.Info("Received Kill Task: " + taskId)
	//TODO: 依据id删除指定的task
	exec.context.ShutDownDomain(taskId)
	exec.context.KillDomain(taskId)
	return nil
}

func (exec *Executor) Message(message []byte) {
	log.Info("Received Message")
}

func (exec *Executor) Error(err string) {
	log.Info("Received Error: " + err)
}

func (exec *Executor) Shutdown() {
	exec.driver.Close()
	log.Info("Received Shutdown")
}

func (exec *Executor) ListenOnVmState() {
	//监听vm状态变化
	for {
		res := <-exec.context.State
		taskId := ""
		for task, dom := range exec.context.Domain { //DomainId反查taskId
			if uuid, _ := dom.Domain.GetUUIDString(); uuid == res.DomainId {
				taskId = task
				break
			}
		}
		switch res.Event.Event {
		case libvirt.DOMAIN_EVENT_RESUMED:
			log.Info("vm is running")
			exec.caller.CallTaskRunning(taskId)
		case libvirt.DOMAIN_EVENT_SUSPENDED:
			log.Info("vm is paused")
			//TODO 调用libvirt暂停虚拟机
			exec.caller.CallTaskFinished(taskId)
		case libvirt.DOMAIN_EVENT_STOPPED:
			//TODO VM停止状态 需要调用mesos task的那个状态？
			log.Infof("vm is stopped")
			exec.caller.CallTaskFinished(taskId)
		case libvirt.DOMAIN_EVENT_CRASHED:
			log.Warning("vm is stopped")
			exec.caller.CallTaskFailed(taskId, "vm failed to start...")
		default:
			log.Infof("vm is unknown state")
			exec.caller.CallMessage([]byte("Unknown vm state"))
		}
	}
}
