package main

import (
	executor "cke/executor"
	//"flag"
	//"io/ioutil"
	"cke"
	"cke/log"
	"cke/vm/exec"
	"os"
)

// var (
//     testThreads = flag.Int("threads", 20, "test thread count")
//     testLoop    = flag.Int("loop", 10, "loop count for one test thread")
//     refreshSec  = flag.Int("refresh", 2, "seconds of refresh data")
// )

func main() {

	// res2, err := createDir("/opt/cke/log")
	// if res2 == false {
	//     panic(err)
	// }
	// file, _ := os.OpenFile("/opt/cke/log/cke-exec-vm.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	// defer file.Close()

	// log.SetOutput(file) //设置输出流
	//log.SetPrefix("[Error]")                            //日志前缀
	//log.SetFlags(log.Llongfile | log.Ldate | log.Ltime) //日志输出样式
	log.Infof("Starting cke-vm-exec Version %s (%s)...", cke.VERSION, cke.BUILD)

	//flag.Parse()

	url := os.Getenv("MESOS_AGENT_ENDPOINT")
	fwId := os.Getenv("MESOS_FRAMEWORK_ID")
	execId := os.Getenv("MESOS_EXECUTOR_ID")

	if len(url) <= 0 {
		log.Error("Missing environment: MESOS_AGENT_ENDPOINT")
		os.Exit(-1)
	}

	if len(fwId) <= 0 {
		log.Error("Missing environment: MESOS_FRAMEWORK_ID")
		os.Exit(-1)
	}

	if len(fwId) <= 0 {
		log.Error("Missing environment: MESOS_EXECUTOR_ID")
		os.Exit(-1)
	}

	log.Infof("url: %s, fwId: %s, execId : %s", url, fwId, execId)

	execImpl, err1 := exec.NewExecutor()
	if err1 != nil {
		log.Error("Error:", err1)
	}
	vmExec := executor.NewExecutor(execImpl)
	err := vmExec.Start(url, fwId, execId)

	if err != nil {
		log.Error("Register mesos framework error:", err)
		return
	}

	log.Info("OK.")
}
