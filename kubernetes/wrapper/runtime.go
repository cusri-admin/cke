package wrapper

import (
	"cke/kubernetes"
	"cke/log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var (
	startIndex int = 0
)

type processRuntime struct {
	waittingExit sync.WaitGroup //等待进程退出的锁
	nodeId       string
	socketPath   string
	taskId       string
	info         *kubernetes.ProcessInfo
	process      *os.Process
	stdout       *os.File
	stderr       *os.File
	sandBox      string
	startIndex   int
}

func createProcessRuntime(nodeId string, socketPath string, outputPath string, taskId string, info *kubernetes.ProcessInfo) (*processRuntime, error) {
	//创建sandbox目录
	sandBox := outputPath + "/" + taskId
	if _, err := os.Stat(sandBox); err != nil {
		if err := os.MkdirAll(sandBox, 0755); err != nil {
			return nil, err
		}
	}

	//打开stdout,stderr的接收文件
	//TODO需要做日志分割的工作
	stdout, err := os.OpenFile(sandBox+"/stdout", os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}
	stderr, err := os.OpenFile(sandBox+"/stderr", os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}
	startIndex++
	return &processRuntime{
		nodeId:     nodeId,
		socketPath: socketPath,
		taskId:     taskId,
		info:       info,
		stdout:     stdout,
		stderr:     stderr,
		sandBox:    sandBox,
		startIndex: startIndex,
	}, nil
}

func (pr *processRuntime) start() error {
	log.Infof("start process(%s): %s", pr.taskId, pr.info.Cmd)

	pr.waittingExit.Add(1) //开始等待

	//输出重定向
	attr := &os.ProcAttr{
		//Dir:   pr.info.WorkPath,
		Env:   []string{"PATH=" + os.Getenv("PATH")}, //出于安全考虑采用最小原则，不需要赋os.Environ(),
		Files: []*os.File{ /*os.Stdin*/ nil, pr.stdout, pr.stderr},
	}

	if pr.info.Envs != nil {
		attr.Env = append(attr.Env, pr.info.Envs...)
	}

	//在环境变量里添加nodeId和executor socket path
	attr.Env = append(attr.Env, "CKE_NODE_ID="+pr.nodeId)
	attr.Env = append(attr.Env, "CKE_EXEC_SOCKET="+pr.socketPath)
	attr.Env = append(attr.Env, "KUBECONFIG=/root/.kube/config")

	var p *os.Process = nil
	var err error = nil
	if strings.Compare("chroot", pr.info.Cmd) == 0 {
		//在第一个参数需要添加程序的全路径
		command := exec.Command(pr.info.Cmd, pr.info.Args...)
		p, err = os.StartProcess(command.Path, command.Args, attr)
	} else {
		//在第一个参数需要添加程序的全路径
		args := append([]string{pr.info.Cmd}, pr.info.Args[0:]...)
		p, err = os.StartProcess(pr.info.Cmd, args, attr)
	}

	if err != nil {
		pr.waittingExit.Add(-1) //结束等待
		return err
	}

	pr.process = p

	return nil
}

func (pr *processRuntime) runWaitting(taskStopped func(taskId string, code kubernetes.WrapperEventResponseStatType, errMsg string)) {
	p := pr.process

	pstat, err := p.Wait()
	pr.process = nil
	if err != nil {
		log.Errorf("Process Wait error: %s\n", err.Error())
		taskStopped(pr.taskId, kubernetes.WrapperEventResponse_FAILED, err.Error())
	} else {
		log.Infof("Process(%s) exited, pstat.string: %s", pr.taskId, pstat.String())
		p.Release()
		//TODO: 修改为go1.12的 PrcessState.ExitCode()
		if pstat.Success() {
			taskStopped(pr.taskId, kubernetes.WrapperEventResponse_FINISHED, "FINISHED")
		} else if strings.Contains(pstat.String(), "signal") {
			taskStopped(pr.taskId, kubernetes.WrapperEventResponse_KILLED, pstat.String())
		} else {
			taskStopped(pr.taskId, kubernetes.WrapperEventResponse_FAILED, pstat.String())
		}
	}
	pr.waittingExit.Add(-1) //结束等待
}

func (pr *processRuntime) kill() {
	p := pr.process

	if p != nil {
		//向进程发送退出通知
		p.Signal(os.Interrupt)
		//如果在5秒后没有退出则直接杀死
		timer := time.AfterFunc(time.Duration(5)*time.Second, func() {
			p.Kill()
		})
		defer timer.Stop()

		//等待自己退出或被杀死
		pr.waittingExit.Wait()
	}
}

func (pr *processRuntime) destory() {
	pr.kill()
	pr.stdout.Close()
	pr.stderr.Close()
}
