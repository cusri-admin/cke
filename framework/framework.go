package framework

import (
	"bytes"
	"cke/log"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	mesos "github.com/mesos/mesos-go/api/v1/lib"
	scheduler "github.com/mesos/mesos-go/api/v1/lib/scheduler"
)

//Framework CKE framework的对应结构
type Framework struct {
	stopLock          sync.WaitGroup //等待http server停止的锁
	roles             []string
	name              string
	httpClient        *http.Client
	resp              *http.Response
	isRunning         bool
	FrameworkID       *mesos.FrameworkID
	heartbeatInterval float64
	receiver          func(event *scheduler.Event)
	mesosStreamID     string
	masters           []string
	currMasterIndex   int
	currMaster        string
	failoverTimeout   float64
	delayFailure      int64
	retryCount        int64
	webURL            string
}

func newFramework(
	frameworkName string,
	roles []string,
	masters []string,
	fwID *string,
	webURL string,
	failover int64,
	delayFailure int64,
	retryCount int64) *Framework {
	fw := &Framework{
		roles:             roles,
		name:              frameworkName,
		masters:           masters,
		heartbeatInterval: 10,
		currMasterIndex:   0,
		failoverTimeout:   float64(failover),
		delayFailure:      delayFailure,
		retryCount:        retryCount,
		webURL:            webURL,
	}
	if fwID != nil {
		fw.FrameworkID = &mesos.FrameworkID{
			Value: *fwID,
		}
	}
	return fw
}

func (fw *Framework) run() error {
	err := fw.connect()
	if err != nil {
		return err
	}
	fw.isRunning = true
	return fw.runLoop()
}

func (fw *Framework) runLoop() (err error) {
	fw.stopLock.Add(1)
	defer fw.stopLock.Done()
	for fw.isRunning {
		err = fw.loopEvents()
		if err == nil {
			log.Info("mesos framework stopped.")
			return
		}
		log.Error("mesos framework http loop error:", err)

		err = fw.tryConnect()
		if err != nil {
			return errors.New("Try to connect mesos many times")
		}
	}
	return
}

func (fw *Framework) readEventLen() (int64, error) {
	lenBuf := make([]byte, 64)
	index := 0
	for true {
		_, err := fw.resp.Body.Read(lenBuf[index : index+1])
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
	eventLen, err := strconv.ParseInt(string(lenBuf[0:index]), 10, 64)
	if err != nil {
		return -1, err
	}
	return eventLen, nil
}

func (fw *Framework) readEventBody(eventLen int64) ([]byte, error) {
	buffer := make([]byte, eventLen)
	_, err := fw.resp.Body.Read(buffer[0:eventLen])
	if err != nil {
		return nil, err
	}
	return buffer, nil
}

func (fw *Framework) readEvent() (event *scheduler.Event, err error) {
	timer := time.AfterFunc(time.Duration(fw.heartbeatInterval*3)*time.Second, func() {
		fw.close()
	})
	defer timer.Stop()

	eventLen, err := fw.readEventLen()
	if err != nil {
		return
	}
	eventByte, err := fw.readEventBody(eventLen)
	if err != nil {
		return
	}

	event = &scheduler.Event{}
	//解码数据
	if err := proto.Unmarshal(eventByte, event); err != nil {
		return nil, err
	}

	return event, nil
}

func (fw *Framework) tryConnect() (err error) {
	var reConnCount int64 = 0
	connected := false
	for !connected && fw.isRunning {
		err = fw.connect()
		if err == nil {
			connected = true
		} else {
			log.Error("mesos framework http connect error:", err)
			time.Sleep(time.Duration(fw.delayFailure) * time.Second)

			reConnCount++
			if reConnCount >= fw.retryCount {
				err = errors.New("Try to connect mesos many times")
				break
			}
		}
	}
	return
}

func (fw *Framework) connect() (err error) {
	fw.currMaster = fw.masters[fw.currMasterIndex]
	fw.currMasterIndex++
	if fw.currMasterIndex >= len(fw.masters) {
		fw.currMasterIndex = 0
	}

	fw.httpClient = &http.Client{}

	user := "root"
	checkPoint := true
	subscribe := &scheduler.Call_Subscribe{
		FrameworkInfo: &mesos.FrameworkInfo{
			ID:              fw.FrameworkID,
			Name:            fw.name,
			User:            user,
			FailoverTimeout: &fw.failoverTimeout,
			WebUiURL:        &fw.webURL,
			Roles:           fw.roles,
			Checkpoint:      &checkPoint,
			Capabilities: []mesos.FrameworkInfo_Capability{
				{
					Type: mesos.FrameworkInfo_Capability_MULTI_ROLE,
				},
			},
		},
	}

	call := &scheduler.Call{
		Type:      scheduler.Call_SUBSCRIBE,
		Subscribe: subscribe,
	}

	if fw.FrameworkID != nil {
		call.FrameworkID = fw.FrameworkID
		log.Info("registe framework by Id: " + fw.FrameworkID.Value)
	}

	data, err := proto.Marshal(call)
	if err != nil {
		return
	}

	req, err := http.NewRequest("POST", "http://"+fw.currMaster+"/api/v1/scheduler", bytes.NewReader(data))

	if err != nil {
		return
	}

	req.Header.Set("Accept", "application/x-protobuf")
	//req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Type", "application/x-protobuf")

	fw.resp, err = fw.httpClient.Do(req)
	if err != nil {
		return
	}
	if fw.resp.StatusCode != 200 {
		buf, err := ioutil.ReadAll(io.LimitReader(fw.resp.Body, 4*1024))
		if err != nil {
			return errors.New("Error read response")
		}
		return errors.New("Http response status code Error: " + strconv.Itoa(fw.resp.StatusCode) + " body:" + string(buf))
	}
	log.Info("mesos master connected " + fw.currMaster)

	event, err := fw.readEvent()
	if err != nil {
		return
	}

	switch event.Type {
	case scheduler.Event_SUBSCRIBED:
		fw.mesosStreamID = fw.resp.Header.Get("Mesos-Stream-Id")
		fw.FrameworkID = event.Subscribed.FrameworkID
		fw.heartbeatInterval = *event.Subscribed.HeartbeatIntervalSeconds
		AgentManager.initAgentList(fw.currMaster, fw.FrameworkID.Value)
		fw.receiver(event)
		break
	default:
		log.Info("Error get framework id, event.type:", event.Type.String(), "; event content:", event.String())
		err = errors.New("Error get framework id")
		break
	}

	return
}

func (fw *Framework) loopEvents() (err error) {
	for true {
		event, err := fw.readEvent()
		if err != nil {
			return err
		}
		fw.receiver(event)
		//lastTime = time.Now()
	}
	return
}

//CallMaster 调用mesos master的方法
func (fw *Framework) CallMaster(call interface{}) (err error) {

	callReq := &scheduler.Call{
		FrameworkID: fw.FrameworkID,
	}
	switch call.(type) {
	case *scheduler.Call_Accept:
		callReq.Type = scheduler.Call_ACCEPT
		callReq.Accept = call.(*scheduler.Call_Accept)
		break
	case *scheduler.Call_Decline:
		callReq.Type = scheduler.Call_DECLINE
		callReq.Decline = call.(*scheduler.Call_Decline)
		break
	case *scheduler.Call_AcceptInverseOffers:
		callReq.Type = scheduler.Call_ACCEPT_INVERSE_OFFERS
		callReq.AcceptInverseOffers = call.(*scheduler.Call_AcceptInverseOffers)
		break
	case *scheduler.Call_DeclineInverseOffers:
		callReq.Type = scheduler.Call_DECLINE_INVERSE_OFFERS
		callReq.DeclineInverseOffers = call.(*scheduler.Call_DeclineInverseOffers)
		break
	case *scheduler.Call_Revive:
		callReq.Type = scheduler.Call_REVIVE
		callReq.Revive = call.(*scheduler.Call_Revive)
		break
	case *scheduler.Call_Kill:
		callReq.Type = scheduler.Call_KILL
		callReq.Kill = call.(*scheduler.Call_Kill)
		break
	case *scheduler.Call_Shutdown:
		callReq.Type = scheduler.Call_SHUTDOWN
		callReq.Shutdown = call.(*scheduler.Call_Shutdown)
		break
	case *scheduler.Call_Acknowledge:
		callReq.Type = scheduler.Call_ACKNOWLEDGE
		callReq.Acknowledge = call.(*scheduler.Call_Acknowledge)
		break
	case *scheduler.Call_AcknowledgeOperationStatus:
		callReq.Type = scheduler.Call_ACKNOWLEDGE_OPERATION_STATUS
		callReq.AcknowledgeOperationStatus = call.(*scheduler.Call_AcknowledgeOperationStatus)
		break
	case *scheduler.Call_Reconcile:
		callReq.Type = scheduler.Call_RECONCILE
		callReq.Reconcile = call.(*scheduler.Call_Reconcile)
		break
	case *scheduler.Call_ReconcileOperations:
		callReq.Type = scheduler.Call_RECONCILE_OPERATIONS
		callReq.ReconcileOperations = call.(*scheduler.Call_ReconcileOperations)
		break
	case *scheduler.Call_Message:
		callReq.Type = scheduler.Call_MESSAGE
		callReq.Message = call.(*scheduler.Call_Message)
		break
	case *scheduler.Call_Request:
		callReq.Type = scheduler.Call_REQUEST
		callReq.Request = call.(*scheduler.Call_Request)
		break
	case *scheduler.Call_Suppress:
		callReq.Type = scheduler.Call_SUPPRESS
		callReq.Suppress = call.(*scheduler.Call_Suppress)
		break
	default:
		return errors.New("Unknown type " + reflect.TypeOf(call).String() + " in call master")
	}
	data, err := proto.Marshal(callReq)
	if err != nil {
		return
	}

	req, err := http.NewRequest("POST", "http://"+fw.currMaster+"/api/v1/scheduler", bytes.NewReader(data))
	if err != nil {
		return
	}

	req.Header.Set("Mesos-Stream-Id", fw.mesosStreamID)
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Accept", "application/x-protobuf")

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
	log.Debugf("Call Master %s return code: %d", callReq.Type.String(), resp.StatusCode)
	return
}

func (fw *Framework) close() {
	fw.isRunning = false
	if fw.resp != nil {
		fw.resp.Body.Close()
	}
	fw.stopLock.Wait()
	fw.resp = nil
}
