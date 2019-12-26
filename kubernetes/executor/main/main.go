package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"cke"
	"cke/executor"
	ke "cke/kubernetes/executor"
	"cke/log"
	"cke/utils"
	"math/rand"
	"time"
)

var (
	exec *executor.Executor

	dockerPath = flag.String("docker_path", "", "The work path of the docker in the container")
	lxcfsPath  = flag.String("lxcfs", "", "The path of LXCFS. if set this argument LXCFS will be used in KubeNode")
	enableCFS  = flag.Bool("cfs", false, "Enable docker CFS in KubeNode")
	logLevel   = flag.String("log_level", "INFO", "level of output logs")
)

func exitFunc(exitCode int) {
	exec.Stop()
	log.Info("cke-k8s-executor exited")
	os.Exit(exitCode)
}

func main() {
	//创建监听退出chan
	exitChan := make(chan os.Signal)
	//监听指定信号 ctrl+c kill
	signal.Notify(exitChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		for s := range exitChan {
			switch s {
			case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				exitFunc(0)
			}
		}
	}()

	// logFile, _ := os.OpenFile("/opt/cke/log/log.log", os.O_WRONLY|os.O_CREATE|os.O_SYNC, 0755)
	// syscall.Dup2(int(logFile.Fd()), 1)
	// syscall.Dup2(int(logFile.Fd()), 2)

	//log.SetFlags(log.Lshortfile | log.Ldate | log.Ltime) //日志输出样式

	flag.Parse()

	url := os.Getenv("MESOS_AGENT_ENDPOINT")
	fwID := os.Getenv("MESOS_FRAMEWORK_ID")
	execID := os.Getenv("MESOS_EXECUTOR_ID")

	log.SetLevel(log.ToLevel(*logLevel))

	log.Infof("Starting CKE K8s executor ver %s(%s)...", cke.VERSION, cke.BUILD)

	if len(url) <= 0 {
		log.Error("Missing environment: MESOS_AGENT_ENDPOINT")
		os.Exit(-1)
	}

	if len(fwID) <= 0 {
		log.Error("Missing environment: MESOS_FRAMEWORK_ID")
		os.Exit(-1)
	}

	if len(execID) <= 0 {
		log.Error("Missing environment: MESOS_EXECUTOR_ID")
		os.Exit(-1)
	}

	workPath, err := os.Getwd()
	if err != nil {
		log.Error("Get work path error:", err)
		os.Exit(-1)
	}

	socketPat := "/tmp/cke"
	_, err = utils.CreateDir(socketPat)
	if err != nil {
		log.Error("create executor unix socket error:", err)
		os.Exit(-2)
	}

	// var dockerWorkDir string
	// if len(*dockerPath) <= 0 {
	// 	//TODO get dockerd work dir
	// 	dockerWorkDir = "/var/lib/cke/"
	// } else {
	// 	dockerWorkDir = *dockerPath
	// }
	// if !strings.HasSuffix(dockerWorkDir, "/") {
	// 	dockerWorkDir += "/"
	// }
	// dockerWorkDir = utils.AppendPath(dockerWorkDir, execID)
	// _, err = utils.CreateDir(dockerWorkDir)
	// if err != nil {
	// 	log.Error("create docker working dir error:", err)
	// 	os.Exit(-2)
	// }

	rand.Seed(time.Now().UnixNano())

	execImpl, err := ke.NewExecutor(fwID, workPath, socketPat+"/"+execID+".sock", *enableCFS,
		*dockerPath, lxcfsPath, exitFunc)
	if err != nil {
		log.Error("Register mesos framework error:", err)
		os.Exit(-3)
	}
	exec = executor.NewExecutor(execImpl)

	err = exec.Start(url, fwID, execID)
	if err != nil {
		log.Error("Register mesos framework error:", err)
		execImpl.Shutdown()
		os.Exit(-4)
	}
	exec.Stop()
	log.Info("OK.")
}
