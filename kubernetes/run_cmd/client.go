package main

import (
	"context"
	"flag"
	"fmt"

	"cke/kubernetes"
	"google.golang.org/grpc"
)

var (
	socketPath = flag.String("socket", "", "Specify the URL of the executor service")
	nodeId     = flag.String("id", "", "Node ID in CKE k8s cluster")
)

func main() {
	flag.Parse()

	conn, err := grpc.Dial("unix://"+*socketPath, grpc.WithInsecure())
	if err != nil {
		fmt.Printf("Connection failed: [%v]\n", err)
		return
	}
	defer conn.Close()
	// 声明客户端
	gRpcClient := kubernetes.NewWrapperProtoClient(conn)
	// 声明 context
	ctx := context.Background()

	execInfo := &kubernetes.ExecInfo{
		NodeId: *nodeId,
		Cmd:    "ls",
		Args:   []string{"-l"},
	}

	fmt.Printf("Exec cmd: [%v]\n", execInfo)

	// 调用
	result, err := gRpcClient.Exec(ctx, execInfo)
	if err != nil {
		fmt.Printf("Failed to exec cmd: [%v]\n", err)
		return
	} else {
		fmt.Printf("Result: %v\n", result)
	}

}
