package conf

import "fmt"

//CKEConf cke的配置
type CKEConf struct {
	ImageName *string             `yaml:"ImageName"` //镜像名称
	Variables Variables           `yaml:"Variables"` //变量
	NodeConfs map[string]NodeConf `yaml:"NodeConfs"` //节点配置
}

//GetVariable 获得配置变量值
func (c *CKEConf) GetVariable(name string) *string {
	return c.Variables.getVariable(name)
}

//PutVariable 添加或更新一个配置变量
func (c *CKEConf) PutVariable(name string, value *string) {
	c.Variables.putVariable(name, value)
}

//将字符串中的变量转换为常量
func (c *CKEConf) var2Volumn(v string) string {
	return c.Variables.var2Volumn(v)
}

//GetNodeConf 得到指定进程的配置信息，其中的配置变量已经被替换
func (c *CKEConf) GetNodeConf(nodeType string) *NodeConf {
	src, ok := c.NodeConfs[nodeType]
	if !ok {
		return nil
	}
	tag := &NodeConf{
		ImageName:    src.ImageName,
		Variables:    newVariable(),
		ProcessConfs: src.ProcessConfs,
		Res:          src.Res,
		cluster:      c,
	}
	for key, value := range src.Variables {
		v := c.var2Volumn(*value)
		tag.Variables[c.var2Volumn(key)] = &v
	}
	return tag
}

//GetImagePath 得到对应的镜像名称
func (c *CKEConf) GetImagePath() string {
	if c.ImageName == nil {
		return ""
	}
	return getImagePath(c.var2Volumn(*c.ImageName))
}

//PrintCKEConf 打印CKEConf结果用于测试
func (c *CKEConf) PrintCKEConf() {
	if c.ImageName != nil {
		fmt.Printf("    ImageName: %s\n", *c.ImageName)
	}
	if len(c.Variables) > 0 {
		fmt.Printf("Variables:\n")
		for key, value := range c.Variables {
			fmt.Printf("  %s: \"%s\"\n", key, *value)
		}
	}
	if len(c.NodeConfs) > 0 {
		fmt.Printf("NodeConfs:\n")
		for name, node := range c.NodeConfs {
			fmt.Printf("  %s:\n", name)
			node.PrintNodeConf()
		}
	}
}
