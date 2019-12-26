package kubernetes

import (
	"encoding/json"
	"strings"
)

//Status 当前运行状态
type Status string

//各种状态
const (
	InitStatus     Status = "init"
	StagingStatus  Status = "staging"
	StartingStatus Status = "starting"
	RunningStatus  Status = "running"
	FaileStatus    Status = "faile"
	KillingStatus  Status = "killing"
	KilledStatus   Status = "killed"
)

//MarshalJSON 将TaskMesosState编码为字符串
func (s *Status) MarshalJSON() ([]byte, error) {
	return json.Marshal(*s)
}

//UnmarshalJSON 从字符串解码TaskMesosState
func (s *Status) UnmarshalJSON(b []byte) error {
	var state string
	json.Unmarshal(b, &state)
	switch Status(strings.ToLower(state)) {
	case RunningStatus:
		*s = RunningStatus
		break
	case StartingStatus:
		*s = StartingStatus
		break
	}
	return nil
}
