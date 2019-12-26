package statistic

import (
	cc "cke/cluster"
	"net/http"

	"github.com/gin-gonic/gin"
)

// StatRestful : Statistic and state infos of cke(includes: scheduler, executor, task etc.)
type StatRestful struct {
	Manager *cc.ClusterManager
	Version string
	Build   string
}

// InitService : initialize a server.
func (ss *StatRestful) InitService(cm *cc.ClusterManager, route *gin.RouterGroup) {
	ss.Manager = cm
	route.GET("/", ss.getState)
}

func (ss *StatRestful) getState(c *gin.Context) {
	c.JSON(http.StatusOK, GetState(ss.Manager, ss.Build, ss.Version))
}
