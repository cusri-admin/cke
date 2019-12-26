package kubernetes

import (
	clr "cke/cluster"
	kc "cke/kubernetes/cluster"
)

//FSMState 描述状态机的一种状态
type FSMState struct {
	Name  string `json:"name"`
	Level int    `json:"level"`
}

//FSMEvent 状态机事件
type FSMEvent struct {
	Type    kc.FSMEventType `json: "type"`
	ProType kc.ProcessType  `json: "processType`
}

//FSMHandler 处理该状态下的业务，并返回新的状态
type FSMHandler func(fsm *kc.FSM, fsmEvent *FSMEvent) (*FSMState, []*clr.TaskInfo)

//FSMInfo 集群状态机信息
type FSMInfo struct {
	State         *FSMState    `json:"state"`
	StartingUp    bool         `json:"starting_up"`
	FinishedState *FSMState    `json:"finished_state"`
	Cluster       *ClusterInfo `json:"cluster"`
	ReleatedNodes []*NodeInfo  `json:"releated_nodes"`
}

//buildFSMState:
func buildFSMState(state *kc.FSMState) *FSMState {
	return &FSMState{
		Name:  state.Name,
		Level: state.Level,
	}
}

//buildFSMEvent :
func buildFSMEvent(evt *kc.FSMEvent) *FSMEvent {
	return &FSMEvent{
		Type:    evt.Type,
		ProType: evt.ProType,
	}
}

//buildFSMInfo : get the cluster fsm.
func buildFSMInfo(cluster *kc.Cluster) *FSMInfo {
	fsmProxy := kc.GetFSMProxy(cluster.GetFSM())
	nodesInfo := make([]*NodeInfo, 0)
	clusterInfo := buildClusterInfo(cluster)
	for _, node := range fsmProxy.ReleatedNodes {
		nodesInfo = append(nodesInfo, buildNodeInfo(node))
	}
	return &FSMInfo{
		// State: &FSMState{Name: tmpFSMState.Name,
		// 	Level: tmpFSMState.Level},
		// ReleadtedNodes: nodesInfo,
		State:         buildFSMState(fsmProxy.State),
		StartingUp:    fsmProxy.StartingUp,
		FinishedState: buildFSMState(fsmProxy.FinishedState),
		Cluster:       clusterInfo,
		ReleatedNodes: nodesInfo,
	}
}
