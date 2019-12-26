package test

import (
	"cke/test"
	"time"
)

func init() {
	test.AddCase(2000, "ha_case", haCase)
}

func haCase(ctx *test.Context) error {

	_, err := ctx.GetClusterNameFromJSON("create_cluster.json")
	if err != nil {
		return err
	}

	if err := ctx.StartOne(0); err != nil {
		ctx.Stop()
		return err
	}

	time.Sleep(time.Duration(2) * time.Second)

	if err := ctx.StartOne(1); err != nil {
		ctx.Stop()
		return err
	}

	time.Sleep(time.Duration(2) * time.Second)

	ctx.StopOne(1)

	// //启动2个scheduler
	// if err := ctx.Start(); err != nil {
	// 	ctx.Stop()
	// 	return err
	// }
	// defer ctx.Stop()

	// time.Sleep(time.Duration(5) * time.Second)

	// //从文件的json中创建集群
	// if err := ctx.CreateCluster("create_cluster.json", ctx.DefaultTimeOut()); err != nil {
	// 	ctx.DeleteCluster(name, 300*1000)
	// 	return err
	// }

	// time.Sleep(time.Duration(2) * time.Second)

	// //删除集群
	// if err := ctx.DeleteCluster(name, 300*1000); err != nil {
	// 	return err
	// }

	return nil
}
