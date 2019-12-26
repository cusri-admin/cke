package conf

import (
	"cke/kubernetes"
	"fmt"
)

//ProcessConf 定义每一个进程的配置
type ProcessConf struct {
	ImageName *string                        `yaml:"ImageName"` //镜像名称
	Files     map[string]*kubernetes.K8SFile //该进程的文件
	Res       *ResourceConf                  `yaml:"res"`  //使用的资源
	Cmd       string                         `yaml:"cmd"`  //命令
	Args      []*Parameter                   `yaml:"args"` //命令行参数
	Envs      []*Parameter                   `yaml:"envs"` //命令的环境变量
	node      *NodeConf                      //所属节点
}

//var2Volumn 将字符串中的变量转换为常量,先在node级别转换，然后在cluster级别转换
func (pc *ProcessConf) var2Volumn(v string) string {
	return pc.node.var2Volumn(v)
}

//SetArg 更新ProcessConf中的一个Argument的设置
//如果配置中允许更新的话
//如果当前配置中没有找到对应的项，则添加一项
func (pc *ProcessConf) SetArg(key, value string) {
	setParameter(key, value, &pc.Args)
}

//GetArg 获得ProcessConf中的一个Argument的设置
func (pc *ProcessConf) GetArg(key string) string {
	return getParameter(key, &pc.Args)
}

//SetEnv 更新ProcessConf中的一个Environment的设置
//如果配置中允许更新的话
//如果当前配置中没有找到对应的项，则添加一项
func (pc *ProcessConf) SetEnv(key, value string) {
	setParameter(key, value, &pc.Envs)
}

//GetEnv 获得ProcessConf中的一个Environment的设置
func (pc *ProcessConf) GetEnv(key string) string {
	return getParameter(key, &pc.Envs)
}

//GetRes 获得ProcessConf中的Resource的设置
func (pc *ProcessConf) GetRes() *ResourceConf {
	if pc.Res == nil {
		return nil
	}
	return &ResourceConf{
		CPU:  pc.Res.CPU,
		Mem:  pc.Res.Mem,
		vars: pc,
	}
}

//GetImagePath 得到对应的镜像名称
func (pc *ProcessConf) GetImagePath() string {
	if pc.ImageName == nil {
		if pc.node == nil {
			return ""
		}
		return pc.node.GetImagePath()
	}
	return getImagePath(*pc.ImageName)
}

//PrintProcConf 打印ProcessConf结果用于测试
func (pc *ProcessConf) PrintProcConf() {
	fmt.Printf("        cmd: %s\n", pc.Cmd)
	if len(pc.Args) > 0 {
		fmt.Printf("        args:\n")
		for _, arg := range pc.Args {
			fmt.Printf("          %+v:\n", arg)
		}
	}
	if len(pc.Envs) > 0 {
		fmt.Printf("        envs:\n")
		for _, env := range pc.Envs {
			fmt.Printf("          %+v:\n", env)
		}
	}
	if pc.GetRes() != nil {
		fmt.Printf("        res:")
		pc.GetRes().PrintResConf()
	}
}
