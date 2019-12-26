package kubernetes

import (
	fmt "fmt"

	"github.com/golang/protobuf/proto"
)

func printProcess(process *ProcessInfo) {
	fmt.Printf("  Cmd: %s\n", process.Cmd)
	fmt.Printf("  WorkPath: %s\n", process.WorkPath)
	fmt.Printf("  Args:\n")
	for _, arg := range process.Args {
		fmt.Printf("    %s\n", arg)
	}
	fmt.Printf("  Envs:\n")
	for _, env := range process.Envs {
		fmt.Printf("    %s\n", env)
	}
	for _, file := range process.Files {
		fmt.Printf("    (%d)%s\n", file.Mode, file.Path)
	}
}

func printNode(node *K8SNodeInfo) {
	fmt.Printf("  Network: %s\n", node.Network)
	fmt.Printf("  Ip: %s\n", node.Ip)
	fmt.Printf("  DnsRecord\n")
	for _, record := range node.DnsRecord {
		fmt.Printf("    %s\t%s\n", record.Host, record.Ip)
	}
	fmt.Printf("  Files:\n")
	for _, file := range node.Files {
		fmt.Printf("    (%d)%s\n", file.Mode, file.Path)
	}
}

//PrintK8STaskByBytes 反序列化，然后打印k8s的task
func PrintK8STaskByBytes(taskData []byte) {
	task := &K8STask{}
	err := proto.Unmarshal(taskData, task)
	if err != nil {
		fmt.Printf("      task: error %s\n", err.Error())
		return
	}
	PrintK8STask(task)
}

//PrintK8STask 打印k8s的task
func PrintK8STask(task *K8STask) {
	fmt.Printf("NodeId: %s, TaskType: %s\n", task.NodeId, task.TaskType.String())
	switch task.TaskType {
	case K8STaskType_START_NODE:
		{
			printNode(task.Node)
			break
		}
	case K8STaskType_START_PROCESS:
		{
			printProcess(task.Process)
			break
		}
	}
}
