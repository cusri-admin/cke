package statistic

import (
	cc "cke/cluster"
)

// State : expose the real state for one process.
type State struct {
	Version     string    `json:"version"`
	Build       string    `josn:"build"`
	FrameworkID string    `json:"framework_id"`
	ProcParams  *Commands `json:"process_args"`
	Env         *Environ  `json:"environ"`
}

//GetState : return the current state of scheduler.
func GetState(cm *cc.ClusterManager, build string, version string) *State {
	return &State{
		Build:       build,
		Version:     version,
		ProcParams:  GetParams(),
		Env:         GetEnv(),
		FrameworkID: cm.GetFrameworkID(),
	}
}
