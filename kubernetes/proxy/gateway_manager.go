package proxy

import (
	"cke/log"
	"net/http"
	"os/exec"

	"github.com/gin-gonic/gin"
)

func NewGatewayManager(svcGateway *string, clusterIPRange *string) *GatewayManager {
	gatewayMananger := &GatewayManager{
		svcGateway:     svcGateway,
		clusterIPRange: clusterIPRange,
		httpRoute:      gin.Default(),
	}
	gatewayMananger.addRouteToHost()
	gatewayMananger.httpRoute.POST("/gateway", func(c *gin.Context) {
		nodeIps := []*string{}
		if err := c.ShouldBindJSON(&nodeIps); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		gatewayMananger.UpdateGateway(nodeIps)
	})

	return gatewayMananger
}

//用于管理k8s集群路由信息，路由信息可能随着k8s集群节点删除变化
type GatewayManager struct {
	svcGateway     *string
	clusterIPRange *string
	httpRoute      *gin.Engine
}

//@param nodeIps表示当前正在运行的k8s节点的node ip（胖容器ip)
func (gm *GatewayManager) UpdateGateway(nodeIps []*string) {
	contains := false
	if gm.svcGateway != nil {
		for _, nodeIp := range nodeIps {
			if *nodeIp == *gm.svcGateway {
				contains = true
			}
		}
	}
	//如果旧的网关已经
	if !contains && len(nodeIps) > 0 {
		gm.deleteRouteToHost()
		gm.svcGateway = nodeIps[0]
		gm.addRouteToHost()
	}
}

func (gm *GatewayManager) deleteRouteToHost() {
	route := gm.getRoute()
	if route == nil {
		return
	}
	deleteCmd := "ip route delete " + *route
	cmd := exec.Cmd{
		Path: "/bin/sh",
		Args: []string{"/bin/sh", "-c", deleteCmd},
	}
	log.Infof("delete route from host, cmd: %s", deleteCmd)
	if err := cmd.Run(); err != nil { // 运行命令
		log.Errorf("execute commond failed when delete route, msg: %s", err.Error())
	}
}

func (gm *GatewayManager) addRouteToHost() {
	route := gm.getRoute()
	if route == nil {
		return
	}
	addCmd := "ip route add " + *route
	cmd := &exec.Cmd{
		Path: "/bin/sh",
		Args: []string{"/bin/sh", "-c", addCmd},
	}
	log.Infof("add route to host, cmd: %s", addCmd)
	if err := cmd.Run(); err != nil { // 运行命令
		log.Errorf("execute commond failed when add route, msg: %s", err.Error())
	}

}

func (gm *GatewayManager) getRoute() *string {
	if gm.svcGateway == nil || *gm.svcGateway == "" {
		return nil
	}

	route := *gm.clusterIPRange + " via " + *gm.svcGateway
	return &route
}

func (cm *GatewayManager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cm.httpRoute.ServeHTTP(w, r)
}
