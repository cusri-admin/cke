package main

import (
	"flag"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"cke"
	"cke/cluster"
	"cke/framework"
	"cke/http"
	k "cke/http/kubernetes"
	stat "cke/http/statistic"
	vm "cke/http/vm"
	"cke/log"
	"cke/storage"
)

var (
	storageType    = flag.String("storage_type", "empty", "Specify the storage type( eg: \"etcd\"、\"zookeeper\",default \"etcd\") of the scheduler, do not use if it is empty")
	storageServers = flag.String("storage_servers", "", "Specify the addresses of storage servers of the scheduler, must be used in conjunction with \"storage_type\", do not use if it is empty")
	frameworkName  = flag.String("name", "cke-framework", "Framework's name")
	httpAddr       = flag.String("listen", "0.0.0.0:8512", "Http service listen address")
	roles          = flag.String("roles", "*", "Whitch roles of framework")
	mesosMaster    = flag.String("master", "", "Mesos masters")
	logLevel       = flag.String("log_level", "INFO", "Level of output logs")
	failover       = flag.Int64("failover", 86400*7, "CKE failover timeout of the framework in mesos")
	authURL        = flag.String("auth", "", "Address for certification services")
	serviceHost    = flag.String("service_host", "$HOST", "CKE service export address")
	servicePort    = flag.String("service_port", "$PORT", "CKE service export port")
	frameworkURL   = flag.String("framework_url", "", "Weburl in mesos private framework")
	executorURL    = flag.String("executor_url", "", "CKE executor download url")
	delayFailure   = flag.Int64("mesos_connect_interval", 5, "Delay time at mesos connect failure")
	retryCount     = flag.Int64("mesos_connect_failure_times", 12, "Retry count at mesos connect failure")

	storageCli     storage.Storage
	clusterManager *cluster.ClusterManager
	sch            *framework.Scheduler
	httpServer     *http.HTTPServer
)

func exitFunc(exitCode int) {
	if httpServer != nil {
		httpServer.Stop()
	}
	sch.Close()
	log.Info("Closed mesos link")

	if storageCli != nil {
		storageCli.Close()
		log.Info("Closed storage link")
	}
	log.Info("CKE-Scheduer exited")
	os.Exit(exitCode)
}

func saveFrameworkID(fwID string) {
	if storageCli != nil {
		err := storageCli.SetFrameworkId(fwID)
		if err != nil {
			log.Error("Save framework id error:", err.Error())
			sch.Close()
			log.Info("Closed mesos link")

			if storageCli != nil {
				storageCli.Close()
				log.Info("Closed storage link")
			}
		}
	}
}

//得到CKE对外提供服务的URL
func getServiceURL() string {
	if len(*serviceHost) > 0 && len(*servicePort) > 0 {
		var host, port string
		if strings.HasPrefix(*serviceHost, "$") {
			host = os.Getenv((*serviceHost)[1:])
		} else {
			host = *serviceHost
		}
		if strings.HasPrefix(*servicePort, "$") {
			port = os.Getenv((*servicePort)[1:])
		} else {
			port = *servicePort
		}
		if len(host) > 0 && len(port) > 0 {
			return "http://" + host + ":" + port + "/"
		}
	}
	return httpServer.GetWebURL()
}

//得到CKE对应的executor二进制文件url地址，如果没有配置默认与serviceURL相同
func getExecutorURL() string {
	if len(*executorURL) > 0 {
		if !strings.HasPrefix(*executorURL, "http") {
			*executorURL = "http://" + *executorURL
		}
		if !strings.HasSuffix(*executorURL, "/") {
			*executorURL = *executorURL + "/"
		}
		return *executorURL
	}
	return getServiceURL()
}

func runScheduler(fwID *string) error {
	serviceURL := getServiceURL()
	log.Infof("CKE native web url: %s", serviceURL)
	var webURL string
	if len(*frameworkURL) <= 0 {
		webURL = serviceURL
	} else {
		webURL = *frameworkURL
		log.Infof("Framework web ui url: %s", webURL)
	}
	masters := strings.Split(*mesosMaster, ",")
	sch = framework.Initialize(
		*frameworkName,
		roles,
		masters,
		fwID,
		saveFrameworkID,
		clusterManager,
		getExecutorURL(),
		webURL,
		*failover,
		*delayFailure,
		*retryCount)
	return sch.Run()
}

func becomeLeader() {
	cluster.StorageServers = *storageServers
	cluster.StorageType = *storageType
	cluster.FrameworkName = *frameworkName
	etcds := strings.Split(*storageServers, ",")

	var err error
	storageCli, err = storage.CreateStorage(storage.StorageType(strings.ToLower(*storageType)), frameworkName, failover, etcds)
	if err != nil {
		log.Errorf("Error create storage client: %s", err.Error())
		exitFunc(-3)
	}
	leaderID := getServiceURL()
	err = storageCli.BecomeLeader(&leaderID)
	if err != nil {
		log.Errorf("Election error: %s", err.Error())
		exitFunc(-4)
	}
	//设置集群为leader
	var fwID *string
	fwID, err = storageCli.GetFrameworkId()
	if err != nil {
		log.Errorf("get framework id error: %s", err.Error())
		exitFunc(-5)
	}
	if fwID == nil {
		//如果没有老fwid，并且storage中有集群数据，则需要根据新fw id重新生成集群数据
		clusterManager.SetLeader(storageCli, true)
	} else {
		//如果有老fwid, 直接注册
		clusterManager.SetLeader(storageCli, false)
	}

	err = runScheduler(fwID)
	if err != nil {
		log.Errorf("Mesos framework error: %s", err.Error())
	}
	storageCli.Close()
}

func main() {
	//创建监听退出chan
	exitChan := make(chan os.Signal)
	//监听指定信号 ctrl+c kill
	signal.Notify(exitChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM,
		syscall.SIGQUIT, syscall.SIGKILL)

	flag.Parse()
	log.SetLevel(log.ToLevel(*logLevel))

	if len(*mesosMaster) <= 0 {
		log.Error("Can't find mesos masters")
		os.Exit(-1)
	}

	rand.Seed(time.Now().Unix())
	log.Infof("Starting CKE version %s(%s)...", cke.VERSION, cke.BUILD)

	clusterManager = cluster.CreateClusterManager()

	httpServer = http.NewHTTPServer(clusterManager, httpAddr, authURL, func() storage.Storage { return storageCli })
	httpServer.AddRouteService("/k8s/v1", &k.K8sRestful{})
	httpServer.AddRouteService("/vm/v1", &vm.VMRestful{})
	httpServer.AddRouteService("/stat/v1", &stat.StatRestful{
		Version: cke.VERSION,
		Build:   cke.BUILD,
	})
	//判断是否是单机模式
	if len(*storageServers) > 0 {
		go func() {
			becomeLeader()
			exitFunc(0)
		}()
	} else {
		go func() {
			runScheduler(nil)
			exitFunc(0)
		}()
	}

	httpServer.Start()

	for s := range exitChan {
		switch s {
		case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGKILL:
			log.Info("Shutdowning Server ...")
			exitFunc(0)
		}
	}
	log.Info("Shutdown OK.")
}
