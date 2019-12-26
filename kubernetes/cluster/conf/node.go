package conf

import (
	"cke/kubernetes"
	"fmt"
)

//NodeConf 节点配置
type NodeConf struct {
	ImageName    *string                `yaml:"ImageName"`    //镜像名称
	Variables    Variables              `yaml:"Variables"`    //保存变量
	Res          *ResourceConf          `yaml:"res"`          //使用的资源
	ProcessConfs map[string]ProcessConf `yaml:"ProcessConfs"` //命令
	cluster      *CKEConf
}

//var2Volumn 将字符串中的变量转换为常量,先在node级别转换，然后在cluster级别转换
func (n *NodeConf) var2Volumn(v string) string {
	ret := n.Variables.var2Volumn(v)
	return n.cluster.var2Volumn(ret)
}

//GetVariable 获得配置变量值
func (n *NodeConf) GetVariable(name string) *string {
	value := n.Variables.getVariable(name)
	if value == nil {
		return n.cluster.GetVariable(name)
	}
	return value
}

//PutVariable 添加或更新一个配置变量
func (n *NodeConf) PutVariable(name string, value *string) {
	n.Variables.putVariable(name, value)
}

//GetRes 获得NodeConf中的Resource的设置
func (n *NodeConf) GetRes() *ResourceConf {
	if n.Res == nil {
		return nil
	}
	return &ResourceConf{
		CPU:  n.Res.CPU,
		Mem:  n.Res.Mem,
		vars: n,
	}
}

//GetProcessConf 得到指定进程的配置信息，其中的配置变量已经被替换
func (n *NodeConf) GetProcessConf(procType string) *ProcessConf {
	src, ok := n.ProcessConfs[procType]
	if !ok {
		return nil
	}
	tag := &ProcessConf{
		ImageName: src.ImageName,
		Files:     make(map[string]*kubernetes.K8SFile),
		Cmd:       src.Cmd,
		Res:       src.Res,
		node:      n,
	}
	for _, para := range src.Args {
		tag.Args = append(tag.Args, &Parameter{
			Key:         n.var2Volumn(para.Key),
			Value:       n.var2Volumn(para.Value),
			AllowModify: para.AllowModify,
		})
	}
	for _, para := range src.Envs {
		tag.Envs = append(tag.Envs, &Parameter{
			Key:         n.var2Volumn(para.Key),
			Value:       n.var2Volumn(para.Value),
			AllowModify: para.AllowModify,
		})
	}
	return tag
}

//GetImagePath 获得镜像全路径
func (n *NodeConf) GetImagePath() string {
	if n.ImageName == nil {
		if n.cluster == nil {
			return ""
		}
		return n.cluster.GetImagePath()
	}
	return getImagePath(n.var2Volumn(*n.ImageName))
}

//PrintNodeConf 打印NodeConf结果用于测试
func (n *NodeConf) PrintNodeConf() {
	if n.ImageName != nil {
		fmt.Printf("    ImageName: %s\n", *n.ImageName)
	}
	if len(n.Variables) > 0 {
		fmt.Printf("    Variables:\n")
		for key, value := range n.Variables {
			fmt.Printf("      %s=%s\n", key, *value)
		}
	}
	if n.GetRes() != nil {
		fmt.Printf("    Res: ")
		n.GetRes().PrintResConf()
	}
	if len(n.ProcessConfs) > 0 {
		fmt.Printf("    ProcessConfs:\n")
		for name, proc := range n.ProcessConfs {
			fmt.Printf("      %s:\n", name)
			proc.PrintProcConf()
		}
	}
}
