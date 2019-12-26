package kubernetes

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"

	cc "cke/cluster"
	kc "cke/kubernetes/cluster"
	"cke/kubernetes/cluster/conf"
	"cke/utils"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/pkg/errors"
	"golang.org/x/net/websocket"
)

//TODO 可以考虑把websocket抽象，并放入独立go文件中
type wsClient struct {
	id     string
	msg    chan interface{}
	ws     *websocket.Conn
	server *K8sRestful
	doneCh chan bool
}

//K8sRestful 保存websocket客户端session
type K8sRestful struct {
	lock      sync.RWMutex // readers对象锁
	cm        *cc.ClusterManager
	pattern   string
	wsClients map[string]*wsClient
	errCh     chan error
}

//InitService 初始化websocket session
func (s *K8sRestful) InitService(
	cm *cc.ClusterManager, route *gin.RouterGroup) {

	s.wsClients = make(map[string]*wsClient)
	s.cm = cm

	route.GET("/help", s.help)
	route.GET("/supported_ver", s.supportedVersions)
	route.GET("/clusters", s.getClusterList)
	route.GET("/clusters/:name", s.getClusterInfo)
	route.GET("/clusters/:name/state", s.getClusterFSM)
	route.POST("/clusters", s.addCluster)
	route.GET("/clusters/:name/adminToken", s.fetchAdminToken)
	route.DELETE("/clusters/:name", s.removeCluster)
	route.POST("/clusters/:name/nodes", s.addNodes)
	route.GET("/clusters/:name/nodes/:node", s.getNode)
	route.DELETE("/clusters/:name/nodes/:node", s.deleteNodes)
	route.PUT("/clusters/:name/nodes/:node/processes/:process", s.updateProcess)
	route.GET("/clusters/:name/logs", s.getLogs)
	route.GET("/clusters/:name/nodes/:node/processes/:proc/files/:file",
		s.readFile)

	//请使用/clusters/:name/console/kubectl,代码里已限制死
	route.GET("/clusters/:name/console/:name1", s.GetGottyServer)
	route.GET("/clusters/:name/console/:name1/:name2", s.GetGottyServer)
	route.GET("/clusters/:name/dashboard", s.GetDashboardServer)

	route.GET("/clusters/:name/json", s.getClusterJsonMsg)
}

func (s *K8sRestful) getClisters() {
	s.cm.GetClustersByType(reflect.TypeOf(&kc.Cluster{}))
}

func (s *K8sRestful) getClusterList(c *gin.Context) {
	cs := make([]*kc.Cluster, 0)
	for _, i := range s.cm.GetClustersByType(reflect.TypeOf(&kc.Cluster{})) {
		cs = append(cs, i.(*kc.Cluster))
	}
	c.JSON(http.StatusOK, buildClusterItems(cs))
}

//getClusterFSM: 获取集群的状态机
func (s *K8sRestful) getClusterFSM(ctx *gin.Context) {
	clusterName := ctx.Param("name")
	cluster := s.cm.GetCluster(clusterName)
	if cluster == nil {
		ctx.JSON(http.StatusNotFound,
			gin.H{"error": "Cluster" + clusterName + " could not be found."})
		return
	}
	ctx.JSON(http.StatusOK, buildFSMInfo(cluster.(*kc.Cluster)))
}

func (s *K8sRestful) getClusterFromHTTP(c *gin.Context) *kc.Cluster {
	clusterName := c.Param("name")
	clr := s.cm.GetCluster(clusterName)
	if clr == nil {
		c.JSON(http.StatusNotFound,
			gin.H{"error": "Cluster " + clusterName + " could not be found"})
		return nil
	}
	return clr.(*kc.Cluster)
}

func (s *K8sRestful) getClusterNameFromHTTP(c *gin.Context) *string {
	clusterName := c.Param("name")
	return &clusterName
}

func (s *K8sRestful) getKubeNodeFromHTTP(c *gin.Context) *kc.KubeNode {
	clusterName := c.Param("name")
	kubeNodeName := c.Param("node")
	clr := s.cm.GetCluster(clusterName)
	if clr == nil {
		c.JSON(http.StatusNotFound,
			gin.H{"error": "Cluster " + clusterName + " could not be found"})
		return nil
	}
	node := clr.(*kc.Cluster).GetKubeNodeByName(&kubeNodeName)
	if node == nil {
		c.JSON(http.StatusNotFound,
			gin.H{"error": "KubeNode " + kubeNodeName + " could not be found"})
		return nil
	}

	return node
}

func (s *K8sRestful) getClusterInfo(c *gin.Context) {
	cluster := s.getClusterFromHTTP(c)
	if cluster == nil {
		return
	}
	c.JSON(http.StatusOK, buildClusterInfo(cluster))
}

func (s *K8sRestful) addCluster(c *gin.Context) {
	var k8sCluster kc.Cluster
	if err := c.ShouldBindBodyWith(&k8sCluster, binding.JSON); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := s.cm.AddCluster(&k8sCluster)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	} else {
		originalMsg, _ := c.Get(gin.BodyBytesKey)
		s, _ := originalMsg.([]byte)
		k8sCluster.SetOriginalMsg(string(s))
		c.JSON(http.StatusCreated, gin.H{"status": 201, "message": "Add Cluster successfully"})
	}
}

func (s *K8sRestful) removeCluster(c *gin.Context) {
	clusterName := c.Param("name")
	t, err := s.cm.RemoveCluster(clusterName)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	} else if t != nil {
		c.String(http.StatusAccepted, "")
	} else {
		c.JSON(http.StatusNotFound,
			gin.H{"error": "Cluster " + clusterName + " could not be found"})
	}
}

func (s *K8sRestful) addNodes(c *gin.Context) {
	cluster := s.getClusterFromHTTP(c)
	if cluster == nil {
		return
	}
	var kubeNodes []*kc.KubeNode
	if err := c.ShouldBindJSON(&kubeNodes); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	//TODO: 需要区分服务端错误(5XX)和客户端错误(4XX)
	err := cluster.AddKubeNodes(kubeNodes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"Add node to cluster " + cluster.ClusterName + " error": err.Error()})
	} else {
		c.JSON(http.StatusCreated, gin.H{"status": 201, "message": "Add node to cluster " + cluster.ClusterName + " successfully"})
	}
}

func (s *K8sRestful) deleteNodes(c *gin.Context) {
	cluster := s.getClusterFromHTTP(c)
	if cluster == nil {
		return
	}
	nodes := strings.Split(c.Param("node"), ",")
	if _, err := cluster.DeleteKubeNodes(nodes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	} else {
		c.JSON(http.StatusAccepted, gin.H{"status": http.StatusAccepted, "message": "Delete node accepted"})
	}
}

func (s *K8sRestful) getNode(c *gin.Context) {
	cluster := s.getClusterFromHTTP(c)
	if cluster == nil {
		return
	}
	nodeName := c.Param("node")
	if len(nodeName) <= 0 {
		c.JSON(http.StatusNotFound,
			gin.H{"error": "Node " + nodeName + " could not be found in cluster " + cluster.ClusterName + "\n"})
		return
	}
	node := cluster.GetKubeNodeByName(&nodeName)
	if node == nil {
		c.JSON(http.StatusNotFound,
			gin.H{"error": "Node " + nodeName + " could not be found in cluster " + cluster.ClusterName + "\n"})
		return
	}
	c.JSON(http.StatusOK, buildNodeInfo(node))
}

//"/clusters/:name/nodes/:node/processes/:process"
func (s *K8sRestful) updateProcess(c *gin.Context) {
	cluster := s.getClusterFromHTTP(c)
	if cluster == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error:": errors.New("can not find cluster").Error()})
		return
	}

	kubeNode := s.getKubeNodeFromHTTP(c)
	if kubeNode == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error:": errors.New("can not find kubenode").Error()})
		return
	}

	var process *kc.Process
	if err := c.ShouldBindJSON(&process); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := process.Initialize(kubeNode); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	processes := append([]*kc.Process{}, process)

	kubeNode.UpdateProcess(cluster, processes)

	c.JSON(http.StatusOK, gin.H{"status": http.StatusOK, "message": "Update process successfully"})
}

func (s *K8sRestful) fetchAdminToken(c *gin.Context) {
	cluster := s.getClusterFromHTTP(c)
	if cluster == nil {
		return
	}
	c.JSON(http.StatusOK,
		gin.H{"AdminToken": cluster.GetAdminToken()})
}

func (s *K8sRestful) getLogs(c *gin.Context) {
	//c.Request.Header.Add("Origin", "http://localhost:8010")
	handler := websocket.Handler(func(conn *websocket.Conn) {
		s.logListen(c, conn)
	})
	handler.ServeHTTP(c.Writer, c.Request)
}

func (s *K8sRestful) logListen(c *gin.Context, conn *websocket.Conn) {
	cluster := s.getClusterFromHTTP(c)
	if cluster == nil {
		return
	}

	//创建一个连接的客户端
	client := &wsClient{
		ws:     conn,
		server: s,
	}

	s.lock.Lock()
	s.wsClients[client.id] = client //在Server中存入连接对象的信息 Map
	s.lock.Unlock()
	//log.Println("Now Android", len(s.Clients), " connected.")

	client.logListenWrite(c, cluster.CreateLogsReader())
}

func (s *K8sRestful) removeLogsClient(client *wsClient) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.wsClients, client.id)
}

//监听对应客户端的chan信道的写入 有写入则发送
//TODO: 当前为通过写入websocket判断链接的正确性，需改进为相应链接断开消息的机制
//TODO: 需要以json分两级描述日志
//    1.基本状态，来源k8s部署的状态机，描述基本状态并用来组织UI进度条
//    2.每个进程(任务)状态，用来显示详细安装进度，并排查错误
func (c *wsClient) logListenWrite(ctx *gin.Context, reader *kc.LogReader) {
	for {
		msg := reader.Read()
		if msg != nil {
			err := websocket.Message.Send(c.ws, *msg)
			//err := websocket.JSON.Send(c.ws, *msg)
			if err != nil {
				reader.Close()
				c.server.removeLogsClient(c)
				return
			}
		} else {
			c.server.removeLogsClient(c)
			return
		}
	}
}

func (s *K8sRestful) readFile(c *gin.Context) {
	offsetStr := c.DefaultQuery("offset", "1")
	lenStr := c.Query("length")

	offset, err := strconv.ParseUint(offsetStr, 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
	}

	length, err := strconv.ParseUint(lenStr, 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
	}

	cluster := s.getClusterFromHTTP(c)
	if cluster == nil {
		return
	}
	nodeID := c.Param("node")
	processName := c.Param("proc")
	fileName := c.Param("file")
	data, err := cluster.GetProcessStdFile(nodeID, processName, fileName, offset, length)
	if err != nil {
		switch err.(type) {
		case *utils.HTTPError:
			hErr := err.(*utils.HTTPError)
			c.String(hErr.Code, hErr.Description)
		default:
			c.String(http.StatusInternalServerError, err.Error())
		}
		return
	}
	c.Data(http.StatusOK, "application/octet-stream", data)
}

//GetGottyServer 访问gotty server
func (s *K8sRestful) GetGottyServer(c *gin.Context) {
	clusterName := c.Param("name")
	clr := s.cm.GetCluster(clusterName)

	if clr == nil {
		c.String(http.StatusNotFound, "Cluster \""+clusterName+"\" cannot be found !")
		return
	}

	targetIP, err := clr.(*kc.Cluster).GetGottyIp()
	if err == nil {
		target := "http://" + targetIP
		url, err := url.Parse(target)
		if err != nil {
			return
		}

		name1 := c.Param("name1")
		name2 := c.Param("name2")

		var path string = ""
		if strings.Compare(name1, "kubectl") == 0 {
			path = ""
		} else if strings.Compare(name1, "kubectlws") == 0 {
			path = "/ws"
		} else {
			path = "/" + name1
			if name2 != "" {
				path = path + "/" + name2
			}
		}
		c.Request.URL.Path = path
		proxy := httputil.NewSingleHostReverseProxy(url)
		proxy.ServeHTTP(c.Writer, c.Request)
		//http.Redirect(c.Writer, c.Request, target, http.StatusTemporaryRedirect)
	} else {
		c.String(http.StatusNotFound, err.Error())
		return
	}
}

//GetDashboardServer 访问Dashboard
func (s *K8sRestful) GetDashboardServer(c *gin.Context) {
	clusterName := c.Param("name")
	clr := s.cm.GetCluster(clusterName)

	if clr == nil {
		c.String(http.StatusNotFound, "Cluster \""+clusterName+"\" cannot be found !")
		return
	}

	target, err := clr.(*kc.Cluster).GetDashboardIpPort()
	if err == nil {
		//target := "https://" + targetIpPort
		http.Redirect(c.Writer, c.Request, target, http.StatusTemporaryRedirect)
	} else {
		c.String(http.StatusNotFound, err.Error())
		return
	}
}

func (s *K8sRestful) getClusterJsonMsg(c *gin.Context) {
	cluster := s.getClusterFromHTTP(c)
	if cluster == nil {
		return
	}

	c.JSON(http.StatusOK, buildClusterMessage(cluster))
	//c.JSON(http.StatusOK, cluster)
	//c.Header("Content-Type", "application/json")
	//c.String(http.StatusOK, cluster.GetOriginalMsg())
}

func (s *K8sRestful) supportedVersions(c *gin.Context) {
	vers := conf.SupportedVersions()
	c.JSON(http.StatusOK, gin.H{"vers": vers})
}

func (s *K8sRestful) help(c *gin.Context) {
	c.String(http.StatusOK,
		`/help GET
/clusters GET|POST
/clusters/:name GET|DELETE
/clusters/:name/adminToken GET
/clusters/:name/state GET
/clusters/:name/nodes POST
/clusters/:name/nodes/:node DELETE
/clusters/:name/nodes/:node/processes/:proc PUT
/clusters/:name/logs WEBSOCKET
/clusters/:name/nodes/:node/processes/:proc/files/:file GET
/clusters/:name/console/kubectl GET
/clusters/:name/dashboard GET
/clusters/:name/json GET
`)
}
