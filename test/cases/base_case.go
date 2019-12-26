package test

import (
	"cke/test"
	"fmt"
	"time"
)

func init() {
	test.AddCase(1000, "base_case", baseCase)
}

func baseCase(ctx *test.Context) error {

	name, err := ctx.GetClusterNameFromJSON("create_cluster.json")
	if err != nil {
		return err
	}

	if err := ctx.Start(); err != nil {
		ctx.Stop()
		return fmt.Errorf("start cke-scheduler error: %s", err)
	}
	defer ctx.Stop()

	//从文件的json中创建集群
	if err := ctx.CreateCluster("create_cluster.json", ctx.DefaultTimeOut()); err != nil {
		ctx.DeleteCluster(name, 300*1000)
		return err
	}

	time.Sleep(time.Duration(2) * time.Second)

	//从文件的json中添加节点
	nodes, err := addNodes(ctx, name, "add_node.json", ctx.DefaultTimeOut())
	if err != nil {
		ctx.DeleteCluster(name, 300*1000)
		return err
	}

	//删除添加的节点
	if err := deleteNode(ctx, name, nodes[0], ctx.DefaultTimeOut()); err != nil {
		ctx.DeleteCluster(name, 300*1000)
		return err
	}

	//删除集群
	if err := ctx.DeleteCluster(name, 300*1000); err != nil {
		return err
	}

	return nil
}

//addNodes 添加节点
func addNodes(ctx *test.Context, cluster string, jsonFile string, timeOut int64) ([]string, error) {

	nodeNames, err := ctx.GetNodeNamesFromJSON(jsonFile)
	if err != nil {
		return nil, err
	}

	//从文件的json中创建集群
	if err := ctx.PostJSONFile2CKE("/clusters/"+cluster+"/nodes", jsonFile); err != nil {
		return nil, err
	}

	exps := make([]string, 0)
	for _, n := range nodeNames {
		exps = append(exps, ctx.NodeOKString(n))
	}
	exps = append(exps, ctx.ReadyString())

	//等待创建成功
	if err := ctx.WaitingAll(exps, timeOut); err != nil {
		return nil, err
	}
	return nodeNames, nil
}

//deleteNodes 删除节点
func deleteNode(ctx *test.Context, cluster string, node string, timeOut int64) error {

	if err := ctx.DeleteMethod("/clusters/"+cluster+"/nodes/"+node, timeOut); err != nil {
		return err
	}

	exps := []string{
		fmt.Sprintf("Stopped node %s.%s", cluster, node),
		ctx.ReadyString(),
	}

	//等待删除成功
	if err := ctx.WaitingAll(exps, timeOut); err != nil {
		return err
	}
	return nil
}
