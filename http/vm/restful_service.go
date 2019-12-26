package http

import (
	clr "cke/cluster"
	"cke/log"
	"cke/vm"
	"cke/vm/model"
	h "net/http"

	"github.com/gin-gonic/gin"
)

type VMRestful struct {
	cluster *vm.Cluster
}

func (s *VMRestful) InitService(cm *clr.ClusterManager, route *gin.RouterGroup) {
	s.cluster = &vm.Cluster{}
	err := cm.AddCluster(s.cluster)
	if err != nil {
		log.Errorf("Init VM cluster error occured. %s", err)
	}

	route.GET("/vms", s.GetVMs)
	route.GET("/vms/:name", s.GetVM)
	route.POST("/vms", s.CreateVM)
	route.PUT("/vms/:name", s.ModifyVM)
	route.DELETE("/vms/:name", s.RemoveVM)
	route.GET("/overvms", s.GetOverVMs)
}

func (s *VMRestful) GetVMs(c *gin.Context) {
	c.JSON(h.StatusOK, s.cluster.GetTasks())
}

func (s *VMRestful) CreateVM(c *gin.Context) {
	var vmInfo model.LibVm
	if err := c.ShouldBindJSON(&vmInfo); err != nil {
		c.JSON(h.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// var taskInfo *clr.TaskInfo
	state := clr.ExpectRUN
	taskInfo := &clr.TaskInfo{
		Name: vmInfo.Name,
		Type: "vm",
		Host: &vmInfo.Host,
		Body: &vmInfo,
		Res: &clr.Resource{
			CPU:  float64(vmInfo.Cpus),
			Mem:  float64(vmInfo.Mem),
			Disk: 0.0,
		},
		State: &state,
	}

	err := s.cluster.AddTask(taskInfo)

	if err != nil {
		c.JSON(h.StatusBadRequest, gin.H{"error": err.Error()})
	} else {
		c.String(h.StatusCreated, "")
	}
}

func (s *VMRestful) GetVM(c *gin.Context) {
	name := c.Param("name")
	c.JSON(h.StatusOK, s.cluster.GetTask(name))
}

func (s *VMRestful) ModifyVM(c *gin.Context) {
	c.String(h.StatusOK, "message")
}

func (s *VMRestful) RemoveVM(c *gin.Context) {
	taskName := c.Param("name")
	t, err := s.cluster.RemoveTask(taskName)
	if err != nil {
		c.JSON(h.StatusInternalServerError, gin.H{"error": err.Error()})
	}
	if t != nil {
		c.String(h.StatusOK, "")
	} else {
		c.JSON(h.StatusNotFound, gin.H{"error": "Not found"})
	}
}

func (s *VMRestful) GetOverVMs(c *gin.Context) {
	c.JSON(h.StatusOK, s.cluster.GetOverTasks())
}
