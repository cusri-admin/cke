package main

import (
	"bytes"
	con "context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os/exec"
	"syscall"

	"cke/kubernetes"
	"google.golang.org/grpc"
)

var (
	socketPath = flag.String("socket", "", "Specify the URL of the executor service")
	nodeId     = flag.String("id", "", "Node ID in CKE k8s cluster")
)

type Server struct {
}

func (s *Server) EventCall(stream kubernetes.WrapperProto_EventCallServer) error {
	ctx := stream.Context()
	for {
		select {
		case <-ctx.Done():
			fmt.Println("connectation has exited")
			return ctx.Err()
		default:
			return nil
		}
	}
}

func (s *Server) Exec(context con.Context, execInfo *kubernetes.ExecInfo) (*kubernetes.ExecResult, error) {
	if *nodeId != execInfo.NodeId {
		fmt.Printf("Exec cmd error: wrapper id: %s can't be found.\n", execInfo.NodeId)
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
		} else {
			fmt.Printf("KubeNode %s exec error: %s\n", execInfo.NodeId, err.Error())
			return nil, err
		}
	}
	result := &kubernetes.ExecResult{
		Code:   0,
		Result: out.String(),
	}
	return result, nil
}

func main() {
	flag.Parse()
	s := &Server{}

	var err error
	server := grpc.NewServer()
	// 注册 Executor Server
	kubernetes.RegisterWrapperProtoServer(server, s)

	serverAddr, err := net.ResolveUnixAddr("unix", *socketPath)
	if err != nil {
		fmt.Println(err.Error())
	}

	listener, err := net.ListenUnix("unix", serverAddr)
	if err != nil {
		fmt.Println(err.Error())
	}

	if err = server.Serve(listener); err != nil {
		fmt.Println("EXEC Server error: " + err.Error())
	}
}
