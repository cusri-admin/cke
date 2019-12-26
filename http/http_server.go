package http

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"cke"
	clr "cke/cluster"
	"cke/log"

	"cke/storage"

	"github.com/gin-gonic/gin"
)

//RestfulService 定义服务接口
type RestfulService interface {
	InitService(cm *clr.ClusterManager, route *gin.RouterGroup)
}

//HTTPServer 保存http server的数据
type HTTPServer struct {
	addr          *string
	route         *gin.Engine
	authURL       *string
	httpSrv       *http.Server
	cm            *clr.ClusterManager
	getStorageCli func() storage.Storage
}

//NewHTTPServer 函数构造一个HttpSever
func NewHTTPServer(cm *clr.ClusterManager, addr *string, authURL *string, getStorageCli func() storage.Storage) (server *HTTPServer) {
	gin.SetMode(gin.ReleaseMode)
	server = &HTTPServer{
		addr:          addr,
		route:         gin.Default(),
		authURL:       authURL,
		cm:            cm,
		getStorageCli: getStorageCli,
	}
	server.initRoute()
	return
}

//AddRouteService 向服务器添加一组服务路径
func (server *HTTPServer) AddRouteService(rootPath string, ser RestfulService) {
	rootRoute := server.route.Group(rootPath)
	ser.InitService(server.cm, rootRoute)
}

//GetWebURL 获得http server的服务地址
func (server *HTTPServer) GetWebURL() string {
	ipPort := strings.Split(*server.addr, ":")
	url := "http://"
	if strings.Trim(ipPort[0], " ") == "0.0.0.0" {
		host, err := os.Hostname()
		if err != nil {
			log.Error("Get host name error:", err.Error())
		}
		url = url + host
	} else {
		url = url + strings.Trim(ipPort[0], " ")
	}
	if len(ipPort) == 2 {
		url = url + ":" + strings.Trim(ipPort[1], " ")
	}
	return url + "/"
}

func (server *HTTPServer) initRoute() {
	server.route.GET("/ver", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"version": cke.VERSION,
			"build":   cke.BUILD,
		})
	})
	server.route.GET("/stack", func(c *gin.Context) {
		buf := make([]byte, 1<<20)
		size := runtime.Stack(buf, true)
		c.String(200, "%s", buf[:size])
	})
	//日志输出相关
	server.route.GET("/log", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"level": log.GetLevel().String(),
		})
	})
	server.route.POST("/log", func(c *gin.Context) {
		var logLevel struct {
			Level string `json:"level"`
		}
		if err := c.ShouldBindJSON(&logLevel); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		log.SetLevel(log.ToLevel(logLevel.Level))
		log.Infof("Set log level to %s", log.GetLevel().String())
		c.String(http.StatusAccepted, "log level accepted\n")
	})
	//静态路由
	server.route.StaticFS("/ui", http.Dir("html/build/ui"))
	server.route.StaticFS("/exec", http.Dir("html/build/exec"))
}

func (server *HTTPServer) checkBasicAuth(basic string) (bool, error) {
	data := strings.Split(basic, " ")
	if len(data) != 2 && data[0] != "Basic" {
		return false, errors.New("illegal basic data")
	}
	dst, err := base64.StdEncoding.DecodeString(data[1])
	if err != nil {
		return false, err
	}
	auth := strings.Split(string(dst), ":")
	if len(auth) != 2 {
		return false, errors.New("illegal basic data")
	}
	log.Infof("Authorization user: %s, password: %s", auth[0], auth[1])
	return true, nil
}

func (server *HTTPServer) checkOAuth2(cookic string) (bool, error) {
	return true, nil
}

func (server *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//只有鉴权,依赖统一的认证机制在本http server之前的http proxy之前完成认证
	//TODO: 缺少鉴权机制
	if server.getStorageCli() != nil && !server.getStorageCli().IsLeader() {
		w.Header().Set("Location", server.getStorageCli().GetLeader())
		w.WriteHeader(http.StatusTemporaryRedirect)
		return
	}
	pass := false
	if len(*server.authURL) > 0 {
		if len(r.Header["Authorization"]) == 1 {
			var err error
			pass, err = server.checkBasicAuth(r.Header.Get("Authorization"))
			if err != nil {
				log.Errorf("Authorization basic error: %s", err.Error())
			}
		} else if len(r.Header["Cookie"]) > 0 {
			for _, c := range r.Cookies() {
				log.Infof("Auth cookie: %s", c)
			}
			cookie := r.Header.Get("Cookie")
			var err error
			pass, err = server.checkOAuth2(cookie)
			if err != nil {
				log.Errorf("OAuth2 cookie error: %s", err.Error())
			}
		}
	} else {
		pass = true
	}

	if pass {
		if r.URL.EscapedPath() == "/" ||
			strings.HasPrefix(r.URL.EscapedPath(), "/cke/") { //这条匹配是为了满足SPA UI的特性
			http.Redirect(w, r, "/ui", http.StatusFound)
			return
		}
		server.route.ServeHTTP(w, r)
	} else {
		w.Header().Set("WWW-Authenticate", "Basic realm=\"CKE Area\"")
		w.WriteHeader(http.StatusUnauthorized)
	}
}

//Start 方法启动一个http server
func (server *HTTPServer) Start() {
	server.httpSrv = &http.Server{
		Addr:    *server.addr,
		Handler: server,
	}
	go func() {
		// service connections
		if err := server.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("listen: %s", err.Error())
		}
	}()
}

//Stop 停止本http srever
func (server *HTTPServer) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.httpSrv.Shutdown(ctx); err != nil {
		log.Error("HTTP server stop error:", err)
	} else {
		log.Info("HTTP server has stopped.")
	}
}
