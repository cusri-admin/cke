package framework

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"cke/log"
	"cke/utils"

	"github.com/golang/protobuf/proto"
	mesosagent "github.com/mesos/mesos-go/api/v1/lib/agent"
	mesosmaster "github.com/mesos/mesos-go/api/v1/lib/master"
)

const (
	//maxFileSize 最大文件大小
	maxFileSize uint64 = 16 * 1024 * 1024
)

var (
	//AgentManager 节点管理器，保存所有mesos节点
	AgentManager agentList
)

type agent struct {
	agentID     string
	hostname    string
	port        int32
	sandBoxPath string
}

func (a *agent) readFile(filePath string, offset uint64, length uint64) ([]byte, error) {
	if length <= 0 {
		length = maxFileSize
	} else if length > maxFileSize {
		return nil, utils.NewHttpError(http.StatusBadRequest,
			"Length is too large. MAX:"+strconv.FormatUint(maxFileSize, 10))
	}

	path := a.sandBoxPath
	if strings.HasPrefix(filePath, "/") {
		path += filePath
	} else {
		path += "/" + filePath
	}

	log.Debugf("ReadFile: %s", path)

	call := &mesosagent.Call{
		Type: mesosagent.Call_READ_FILE,
		ReadFile: &mesosagent.Call_ReadFile{
			Path:   path,
			Offset: offset,
			Length: &length,
		},
	}

	data, err := proto.Marshal(call)
	if err != nil {
		return nil, err
	}

	respData, err := callMesosHTTP(fmt.Sprintf("%s:%d", a.hostname, a.port), data, int64(length))

	if err != nil {
		return nil, err
	}

	resp := &mesosagent.Response{}
	//解码数据
	if err := proto.Unmarshal(respData, resp); err != nil {
		return nil, err
	}

	if resp.ReadFile == nil {
		return nil, utils.NewHttpError(http.StatusInternalServerError, "Read file error: agent Response_ReadFile is nil.")
	}

	return resp.ReadFile.Data, nil
}

type agentList struct {
	master      string       //mesos masters
	lock        sync.RWMutex // 更新用的锁
	agList      *agentList   //保存所有的agent
	frameworkID string

	isUpdating bool
	agents     map[string]*agent
}

func (al *agentList) initAgentList(masterStr string, fwID string) {
	al.master = masterStr
	al.frameworkID = fwID
	go al.updateAgents()
}

func (al *agentList) updateAgents() {
	//保证只进入一次
	al.lock.Lock()
	if al.isUpdating {
		al.lock.Unlock()
		return
	}
	al.isUpdating = true
	al.lock.Unlock()

	//获得新数据
	list, err := al.getAgentList()
	if err != nil {
		log.Errorf("Update agent info error: %s", err.Error())
		return
	}

	//更新对外数据
	al.lock.Lock()
	defer al.lock.Unlock()
	al.agents = list
}

func (al *agentList) getAgentList() (map[string]*agent, error) {

	call := &mesosmaster.Call{
		Type: mesosmaster.Call_GET_AGENTS,
	}

	data, err := proto.Marshal(call)
	if err != nil {
		log.Errorf("getAgentList proto.Marshal error: %s", err.Error())
		return nil, err
	}

	respData, err := callMesosHTTP(al.master, data, int64(maxFileSize))

	if err != nil {
		return nil, err
	}

	resp := &mesosmaster.Response{}
	//解码数据
	if err := proto.Unmarshal(respData, resp); err != nil {
		log.Errorf("getAgentList proto.Unmarshal error: %s", err.Error())
		return nil, err
	} else if resp.GetAgents == nil {
		return nil, errors.New("Get agent list error: master Response_GetAgents is nil")
	}

	var wg sync.WaitGroup
	agents := make(map[string]*agent)
	for _, ag := range resp.GetAgents.Agents {
		a := &agent{
			agentID:  ag.AgentInfo.ID.Value,
			hostname: ag.AgentInfo.Hostname,
			port:     *ag.AgentInfo.Port,
		}
		agents[a.agentID] = a
		wg.Add(1)
		go al.getAgentSandBox(&wg, a)
	}

	wg.Wait()

	return agents, nil
}

func (al *agentList) getAgentSandBox(wg *sync.WaitGroup, a *agent) {
	defer wg.Done()

	call := &mesosagent.Call{
		Type: mesosagent.Call_GET_FLAGS,
	}

	data, err := proto.Marshal(call)
	if err != nil {
		log.Errorf("Get agent %s(%s:%d) sandbox error: %s", a.agentID, a.hostname, a.port, err.Error())
		return
	}

	respData, err := callMesosHTTP(fmt.Sprintf("%s:%d", a.hostname, a.port), data, int64(maxFileSize))

	if err != nil {
		log.Errorf("Get agent %s(%s:%d) sandbox error: %s", a.agentID, a.hostname, a.port, err.Error())
		return
	}

	resp := &mesosagent.Response{}
	//解码数据
	if err := proto.Unmarshal(respData, resp); err != nil {
		log.Errorf("Get agent %s(%s:%d) sandbox error: %s", a.agentID, a.hostname, a.port, err.Error())
		return
	} else if resp.GetFlags == nil {
		log.Error("Get agent sendbox error: agent Response_GetFlags is nil.")
		return
	}

	for _, flag := range resp.GetFlags.Flags {
		if flag.Name == "work_dir" {
			a.sandBoxPath = *flag.Value + "/slaves/" + a.agentID + "/frameworks/" + al.frameworkID + "/executors"
			log.Debugf("Agent %s(%s:%d) sandbox: %s", a.agentID, a.hostname, a.port, a.sandBoxPath)
		}
	}
}

func (al *agentList) ReadAgentFile(agentID string, filePath string,
	offset uint64, length uint64) ([]byte, error) {

	al.lock.RLock()
	a := al.agents[agentID]
	al.lock.RUnlock()

	if a == nil {
		go al.updateAgents()
		return nil, utils.NewHttpError(http.StatusNotFound, "Get agent file error: agent "+agentID+" could not be found.")
	}
	//filePath := "/" + p.Task.Info.Executor.ID + "/runs/latest/" + nodeID + "/" + p.Task.Status.Id + "/" + fileName
	return a.readFile(filePath, offset, length)
}
