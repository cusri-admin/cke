package executor

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cke/cluster"
	"cke/log"

	"github.com/golang/protobuf/proto"
	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/executor"
	"github.com/pborman/uuid"
	"os"
)


type Caller interface {
	CallTaskStarting(taskId string) error
	CallTaskRunning(taskId string) error
	CallTaskFinished(taskId string) error
	CallTaskKilled(taskId string) error
	CallTaskFailed(taskId string, errMsg string) error
	CallMessage(message []byte) error
}

type ExecutorInterface interface {
	UnmarshalTaskBody(taskBody []byte) (task interface{}, err error)
	Started()
	Registered(caller Caller)
	Launch(taskId string, task *TaskInfoWithResource) error
	//LaunchGroup(launchGroup *executor.Event_LaunchGroup)
	Kill(taskId string) error
	Message(message []byte)
	Error(err string)
	Shutdown()
}

type RunningTask struct {
	Task *cluster.TaskInfo
	Info mesos.TaskInfo
}

type Executor struct {
	url                      string
	httpClient               *http.Client
	resp                     *http.Response
	isRunning                bool
	heartbeatIntervalSeconds float64
	ExecutorInfo             mesos.ExecutorInfo
	FwId                     mesos.FrameworkID
	ExecId                   mesos.ExecutorID
	unackedTasks             map[mesos.TaskID]mesos.TaskInfo
	unackedUpdates           map[string]executor.Call_Update
	failedTasks              map[mesos.TaskID]mesos.TaskStatus // send updates for these as we can
	execImpl                 ExecutorInterface
	mesosSubscribeBackoffMax  int
	//runningTasks             map[string]*RunningTask
}

func NewExecutor(execImpl ExecutorInterface) *Executor {
	
	return &Executor{
		unackedTasks:   make(map[mesos.TaskID]mesos.TaskInfo),
		unackedUpdates: make(map[string]executor.Call_Update),
		execImpl:       execImpl,
	}
}

func fetchSubscribeBackOff() int {
	mesosBackOff := os.Getenv("MESOS_SUBSCRIPTION_BACKOFF_MAX")

	if strings.HasSuffix(mesosBackOff, "ms"){
		if backOff, err := strconv.Atoi(strings.TrimSuffix(mesosBackOff, "ms")); err == nil{
			return backOff
		}
		return 2000
	}

	if strings.HasSuffix(mesosBackOff, "secs"){
		if backOff, err := strconv.Atoi(strings.TrimSuffix(mesosBackOff, "secs")); err == nil{
			return backOff * 1000
		}
		return 2000
	}

	if strings.HasSuffix(mesosBackOff, "mins"){
		if backOff, err := strconv.Atoi(strings.TrimSuffix(mesosBackOff, "mins")); err == nil{
			return backOff * 60 * 1000
		}
		return 2000
	}

	return 2000
}

func (exec *Executor) Start(url string, frameworkId string, execId string) (err error) {
	exec.url = url
	exec.heartbeatIntervalSeconds = 10
	exec.isRunning = true
	exec.ExecId = mesos.ExecutorID{
		Value: execId,
	}
	exec.FwId = mesos.FrameworkID{
		Value: frameworkId,
	}
	err = exec.connect()
	if err != nil {
		return
	}
	exec.execImpl.Started()

	exec.run()

	return nil
}

func (exec *Executor) Stop() {
	exec.onShutdown()
}

func (exec *Executor) run() {
	reConnected := false
	for true {
		err := exec.loopEvents()
		if err == nil || !exec.isRunning {
			log.Info("cke executor stopped.")
			break
		}
		log.Error("cke executor http loop error:", err)
		reConnected = false
		for !reConnected {
			err := exec.connect()
			if err == nil {
				reConnected = true
			} else {
				log.Error("cke executor http connect error:", err)
				time.Sleep(time.Duration(fetchSubscribeBackOff()) * time.Millisecond)
			}
		}
	}
}

func (exec *Executor) readEventLen() (int64, error) {
	lenBuf := make([]byte, 64)
	index := 0
	for true {
		_, err := exec.resp.Body.Read(lenBuf[index : index+1])
		if err != nil {
			return -1, err
		}
		if lenBuf[index] == '\n' {
			lenBuf[index] = 0
			break
		}

		index++
		if index >= 64 {
			return -1, errors.New("out of max buffer")
		}
	}
	event_len, err := strconv.ParseInt(string(lenBuf[0:index]), 10, 64)
	if err != nil {
		return -1, err
	}
	return event_len, nil
}

func (exec *Executor) readEventBody(event_len int64) ([]byte, error) {
	buffer := make([]byte, event_len)
	_, err := exec.resp.Body.Read(buffer[0:event_len])
	if err != nil {
		return nil, err
	}
	return buffer, nil
}

func (exec *Executor) readEvent() (event *executor.Event, err error) {
	event_len, err := exec.readEventLen()
	if err != nil {
		return nil, err
	}
	eventByte, err := exec.readEventBody(event_len)
	if err != nil {
		return nil, err
	}

	event = &executor.Event{}
	//解码数据
	err = proto.Unmarshal(eventByte, event)
	if err != nil {
		return nil, err
	}

	return event, nil
}

func (exec *Executor) unacknowledgedTasks() (result []mesos.TaskInfo) {
	if n := len(exec.unackedTasks); n > 0 {
		result = make([]mesos.TaskInfo, 0, n)
		for k := range exec.unackedTasks {
			result = append(result, exec.unackedTasks[k])
		}
	}
	return
}

func (exec *Executor) unacknowledgedUpdates() (result []executor.Call_Update) {
	if n := len(exec.unackedUpdates); n > 0 {
		result = make([]executor.Call_Update, 0, n)
		for k := range exec.unackedUpdates {
			result = append(result, exec.unackedUpdates[k])
		}
	}
	return
}

func (exec *Executor) connect() (err error) {
	exec.httpClient = &http.Client{}

	subscribe := &executor.Call_Subscribe{
		UnacknowledgedTasks:   exec.unacknowledgedTasks(),
		UnacknowledgedUpdates: exec.unacknowledgedUpdates(),
	}

	call := &executor.Call{
		Type:        executor.Call_SUBSCRIBE,
		ExecutorID:  exec.ExecId,
		FrameworkID: exec.FwId,
		Subscribe:   subscribe,
	}

	data, err := proto.Marshal(call)
	if err != nil {
		return
	}

	req, err := http.NewRequest("POST", "http://"+exec.url+"/api/v1/executor", bytes.NewReader(data))
	if err != nil {
		return
	}

	req.Header.Set("Accept", "application/x-protobuf")
	//req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("User-Agent", "CKE-EXEC-"+exec.ExecId.Value)

	exec.resp, err = exec.httpClient.Do(req)
	if err != nil {
		return
	}
	if exec.resp.StatusCode != 200 {
		return errors.New("Http response status code Error: " + strconv.Itoa(exec.resp.StatusCode))
	}
	log.Infof("mesos master connected %s", exec.url)

	event, err := exec.readEvent()
	if err != nil {
		return
	}

	switch event.Type {
	case executor.Event_SUBSCRIBED:
		exec.ExecutorInfo = event.Subscribed.ExecutorInfo
		exec.heartbeatIntervalSeconds = 10.0
		log.Infof("Registered executor: %s", exec.ExecutorInfo.ExecutorID.Value)
		exec.execImpl.Registered(exec)
		break
	default:
		err = errors.New("Error get executor id")
		break
	}

	return
}

func (exec *Executor) loopEvents() (err error) {
	for true {
		event, err := exec.readEvent()
		if err != nil {
			return err
		}
		exec.received(event)
		//lastTime = time.Now()
	}
	return
}

func (exec *Executor) close() {
	exec.resp.Body.Close()
}

func (exec *Executor) received(event *executor.Event) {
	switch event.Type {
	case executor.Event_ACKNOWLEDGED:
		exec.onAcknowledged(event.Acknowledged)
		break
	case executor.Event_LAUNCH:
		exec.onLaunch(event.Launch)
		break
	case executor.Event_LAUNCH_GROUP:
		exec.onLaunchGroup(event.LaunchGroup)
		break
	case executor.Event_KILL:
		exec.onKill(event.Kill)
		break
	case executor.Event_MESSAGE:
		exec.onMessage(event.Message)
		break
	case executor.Event_ERROR:
		exec.onError(event.Error)
		break
	case executor.Event_SHUTDOWN:
		exec.onShutdown()
		break
	default:
		log.Warning("Received unknown event:", event.Type)
		break
	}
}

func (exec *Executor) callAgent(call *executor.Call) (err error) {
	data, err := proto.Marshal(call)
	if err != nil {
		return
	}

	//TODO 修改为长连接
	req, err := http.NewRequest("POST", "http://"+exec.url+"/api/v1/executor", bytes.NewReader(data))
	if err != nil {
		return
	}

	req.Header.Set("Host", exec.url)
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Accept", "application/x-protobuf")
	req.Header.Set("User-Agent", exec.ExecId.Value)

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		buf, err := ioutil.ReadAll(io.LimitReader(resp.Body, 4*1024))
		if err != nil {
			return errors.New("Error read response")
		}
		return errors.New("Http response status code Error: " + strconv.Itoa(resp.StatusCode) + " body:" + string(buf))
	}
	return
}

func (exec *Executor) updateTaskState(taskId string, state *mesos.TaskState, msg *string) error {
	nowTime := float64(time.Now().Unix())
	call := &executor.Call{
		ExecutorID:  exec.ExecId,
		FrameworkID: exec.FwId,
		Type:        executor.Call_UPDATE,
		Update: &executor.Call_Update{
			Status: mesos.TaskStatus{
				TaskID: mesos.TaskID{
					Value: taskId,
				},
				ExecutorID: &exec.ExecId,
				State:      state,
				Message:    msg,
				Source:     mesos.SOURCE_EXECUTOR.Enum(),
				UUID:       []byte(uuid.NewRandom()),
				Timestamp:  &nowTime,
			},
		},
	}

	err := exec.callAgent(call)
	if err != nil {
		log.Error("Call UPDATE error: ", err)
		return err
	} else {
		exec.unackedUpdates[string(call.Update.Status.UUID)] = *call.Update
	}
	return nil
}

func (exec *Executor) onLaunch(launch *executor.Event_Launch) {
	exec.unackedTasks[launch.Task.TaskID] = launch.Task

	log.Info("Received Launch Task: " + launch.Task.TaskID.Value)
	taskId := launch.Task.TaskID.Value
	taskBody, err := exec.execImpl.UnmarshalTaskBody(launch.Task.Data)
	if err != nil {
		log.Error(err)
		msg := err.Error()
		exec.updateTaskState(taskId, mesos.TASK_FAILED.Enum(), &msg)
	} else {
		cpus := 0.0
		mem := 0.0
		for _, res := range launch.Task.Resources {
			if strings.Compare(res.Name, "cpus") == 0 {
				cpus += res.Scalar.Value
			} else if strings.Compare(res.Name, "mem") == 0 {
				mem += res.Scalar.Value
			}
		}

		taskInfoWithResource := &TaskInfoWithResource{
			taskBody,
			cpus,
			mem,
		}

		go func() {
			err := exec.execImpl.Launch(taskId, taskInfoWithResource)
			if err != nil {
				msg := err.Error()
				exec.updateTaskState(taskId, mesos.TASK_FAILED.Enum(), &msg)
			}
		}()
	}
}

func (exec *Executor) onAcknowledged(acknowledged *executor.Event_Acknowledged) {
	log.Debugf("Received Acknowledged Task: %s", acknowledged.TaskID.Value)
	delete(exec.unackedUpdates, string(acknowledged.UUID))
	delete(exec.unackedTasks, acknowledged.TaskID)
}

func (exec *Executor) onLaunchGroup(launchGroup *executor.Event_LaunchGroup) {
	log.Info("Received LaunchGroup")
}

func (exec *Executor) onKill(kill *executor.Event_Kill) {

	log.Infof("Received Kill Task: %s", kill.TaskID.Value)

	taskId := kill.TaskID.Value
	exec.updateTaskState(taskId, mesos.TASK_KILLING.Enum(), nil)
	go func() {
		err := exec.execImpl.Kill(taskId)
		if err != nil {
			log.Error(err)
			msg := err.Error()
			exec.updateTaskState(taskId, mesos.TASK_ERROR.Enum(), &msg)
		}
	}()
}

func (exec *Executor) onMessage(message *executor.Event_Message) {
	exec.execImpl.Message(message.Data)
}

func (exec *Executor) onError(err *executor.Event_Error) {
	exec.execImpl.Error(err.Message)
}

func (exec *Executor) onShutdown() {
	log.Info("Executor onShutdown")
	exec.execImpl.Shutdown()
	exec.isRunning = false
	exec.close()
}

func (exec *Executor) CallTaskStarting(taskId string) error {
	log.Info("task starting: " + taskId)
	return exec.updateTaskState(taskId, mesos.TASK_STARTING.Enum(), nil)
}
func (exec *Executor) CallTaskRunning(taskId string) error {
	log.Info("task running: " + taskId)
	return exec.updateTaskState(taskId, mesos.TASK_RUNNING.Enum(), nil)
}
func (exec *Executor) CallTaskFinished(taskId string) error {
	log.Info("task finished: " + taskId)
	return exec.updateTaskState(taskId, mesos.TASK_FINISHED.Enum(), nil)
}
func (exec *Executor) CallTaskKilled(taskId string) error {
	log.Info("task killed: " + taskId)
	return exec.updateTaskState(taskId, mesos.TASK_KILLED.Enum(), nil)
}
func (exec *Executor) CallTaskFailed(taskId string, errMsg string) error {
	log.Info("task CallFailed: " + taskId + " msg: " + errMsg)
	return exec.updateTaskState(taskId, mesos.TASK_FAILED.Enum(), &errMsg)
}
func (exec *Executor) CallMessage(message []byte) (err error) {
	call := &executor.Call{
		ExecutorID:  exec.ExecId,
		FrameworkID: exec.FwId,
		Type:        executor.Call_MESSAGE,
		Message: &executor.Call_Message{
			Data: message,
		},
	}
	err = exec.callAgent(call)
	if err != nil {
		log.Error("Call UPDATE error: ", err)
	}
	return
}
