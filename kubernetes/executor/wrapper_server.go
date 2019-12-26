package executor

import (
	"bytes"
	con "context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"syscall"

	"cke/kubernetes"
	"cke/log"

	"google.golang.org/grpc"
)

//WrapperServer gRPC server
type WrapperServer struct {
	listener        *net.UnixListener
	isRunning       bool
	serverPath      string
	serverAddr      *net.UnixAddr
	kubeNodeManager KubeNodeManager
}

func createServer(socketPath string) (*WrapperServer, error) {
	socketPath = "wrapper.sock"
	os.Remove(socketPath)
	serverAddr, err := net.ResolveUnixAddr("unix", socketPath)
	if err != nil {
		return nil, err
	}

	return &WrapperServer{
		//getContainer: getContainer,
		serverAddr: serverAddr,
		serverPath: socketPath,
	}, nil
}

func (ws *WrapperServer) getServerPath() string {
	return ws.serverPath
}

func (ws *WrapperServer) start() error {
	var err error
	server := grpc.NewServer()
	// 注册 Executor Server
	kubernetes.RegisterWrapperProtoServer(server, ws)
	ws.listener, err = net.ListenUnix("unix", ws.serverAddr)
	if err != nil {
		return err
	}

	//修改socket权限为只能root读写
	if err := os.Chmod(ws.serverPath, 0600); err != nil {
		ws.listener.Close()
		return err
	}

	go func() {
		if err = server.Serve(ws.listener); err != nil {
			//退出机制待完善
			log.Error("EXEC Server error: " + err.Error())
			os.Exit(-9)
		}
	}()
	return nil
}

//EventCall gRPC服务函数
func (ws *WrapperServer) EventCall(stream kubernetes.WrapperProto_EventCallServer) error {
	ctx := stream.Context()
	for {
		select {
		case <-ctx.Done():
			log.Info("connectation has exited")
			return ctx.Err()
		default:
			// 接收从客户端发来的消息
			wrapperEvent, err := stream.Recv()
			if err == io.EOF {
				log.Info("End of data stream sent by wrapper")
				return nil
			}
			if err != nil {
				log.Error("Error receiving data:", err)
				return err
			}
			// 如果接收正常，则判断该连接是否是合法的wrapper
			if wrapperEvent.Type != kubernetes.WrapperEvent_REGISTERED {
				log.Warning("Wrong registration message")
				return nil
			}
			container := ws.kubeNodeManager.getStartingKubeNode(wrapperEvent.NodeId)
			if container == nil {
				log.Warningf("Connection error: wrapper id: %s can't be found.", wrapperEvent.NodeId)
				return nil
			}
			if err := container.initialize(stream, ctx); err != nil {
				log.Errorf("wrapper(%s) initialize error: %s", wrapperEvent.NodeId, err.Error())
				return err
			}
			ws.kubeNodeManager.moveToRunningKubeNode(wrapperEvent.NodeId)
			err = container.eventLoop(stream, ctx)
			if err != nil {
				log.Errorf("Wrapper(%s) eventLoop return: %s", wrapperEvent.NodeId, err.Error())
			}
			return err
		}
	}
}

//Exec gRPC服务函数
func (ws *WrapperServer) Exec(context con.Context, execInfo *kubernetes.ExecInfo) (*kubernetes.ExecResult, error) {
	container := ws.kubeNodeManager.getRunningKubeNode(execInfo.NodeId)
	if container == nil {
		log.Warningf("Exec cmd error: wrapper id: %s can't be found.", execInfo.NodeId)
		return nil, errors.New("Illegal wrapper")
	}
	//exec cmd
	cmd := exec.Command(execInfo.Cmd, execInfo.Args...)
	cmd.Env = execInfo.Envs
	//cmd.Stdin = strings.NewReader("some input")
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			//TODO: 修改为go1.12版本的
			// if exitError, ok := err.(*exec.ExitError); ok {
			// 	return exitError.ExitCode()
			// }
			ws := exitError.Sys().(syscall.WaitStatus)
			exitCode := int32(ws.ExitStatus())
			result := &kubernetes.ExecResult{
				Code:   exitCode,
				Result: errBuf.String(),
			}
			return result, nil
		}
		log.Errorf("KubeNode %s exec error: %s", execInfo.NodeId, err.Error())
		return nil, err
	}
	result := &kubernetes.ExecResult{
		Code:   0,
		Result: out.String(),
	}
	return result, nil
}

func (ws *WrapperServer) stop() {
	ws.listener.Close()
	os.Remove(ws.serverPath)
}
