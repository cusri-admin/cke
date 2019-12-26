package framework

import (
	"testing"

	"cke/log"
)

func TestUpdateAgents(t *testing.T) {
	log.SetLevel(log.DEBUG)
	master := "10.124.142.222:5050"
	AgentManager.initAgentList(master, "fwId")
	AgentManager.updateAgents()
}
