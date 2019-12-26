package main

import (
	"cke/log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"cke"
	"cke/kubernetes/executor"
	"cke/kubernetes/wrapper"
)

var (
	kw *wrapper.K8sWrapper
)

func exitFunc(exitCode int) {
	log.Info("cke-k8s-wrapper exited")
	kw.KillProcesses()
	os.Exit(exitCode)
}

func main() {
	socket := os.Getenv(executor.EVN_EXEC_ENDPOINT)
	nodeID := os.Getenv(executor.EVN_KUBE_NODE_ID)
	outputPath := os.Getenv(executor.EVN_OUTPUT_PATH)
	heartbeat := os.Getenv(executor.ENV_HEARTBEAT)
	logLevel := os.Getenv(log.ENV_LOG_LEVEL)

	log.SetLevel(log.ToLevel(logLevel))

	stdoutFile := outputPath + "/wrapper.out"
	wrapperStdout, err := os.OpenFile(stdoutFile, os.O_WRONLY|os.O_CREATE|os.O_SYNC, 0644)
	if err != nil {
		log.Errorf("Create stdout file %s error: %s", stdoutFile, err.Error())
		os.Exit(-1)
	}
	syscall.Dup2(int(wrapperStdout.Fd()), 1)

	stderrFile := outputPath + "/wrapper.err"
	wrapperStderr, err := os.OpenFile(stderrFile, os.O_WRONLY|os.O_CREATE|os.O_SYNC, 0644)
	if err != nil {
		log.Errorf("Create stderr file %s error: %s", stderrFile, err.Error())
		os.Exit(-1)
	}
	syscall.Dup2(int(wrapperStderr.Fd()), 2)

	hb, err := strconv.ParseInt(heartbeat, 10, 32)
	if err != nil {
		log.Errorf("heartbeat strconv.ParseInt error: %s", err.Error())
		os.Exit(-1)
	}

	log.Infof("Starting CKE K8s wrapper ver %s(%s) ... log level: %s", cke.VERSION, cke.BUILD, log.GetLevel().String())

	//创建监听退出chan
	exitChan := make(chan os.Signal)
	//监听指定信号 ctrl+c kill
	signal.Notify(exitChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM,
		syscall.SIGQUIT, syscall.SIGUSR1, syscall.SIGUSR2)
	go func() {
		for s := range exitChan {
			switch s {
			case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				exitFunc(0)
			case syscall.SIGUSR1:
				log.Warning("usr1", s)
			case syscall.SIGUSR2:
				log.Warning("usr2", s)
			}
		}
	}()

	kw := wrapper.CreateK8sWrapper(socket, nodeID, outputPath, int32(hb))
	//TODO 网络异常的处理
	tryCount := 0
	for {
		err := kw.Run()
		kw.CloseStream()
		if err != nil {
			log.Error("CKE wrapper error:", err)
			time.Sleep(time.Duration(5) * time.Second)
			tryCount++
			if tryCount >= 4 {
				break
			}
		} else {
			break
		}
	}
	log.Info("cke k8s wrapper stopped")
}
