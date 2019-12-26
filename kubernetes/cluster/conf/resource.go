package conf

import (
	"fmt"
	"strconv"
)

//ResourceConf 描述配置中的资源信息
type ResourceConf struct {
	CPU  string      `yaml:"cpu"` //cpu
	Mem  string      `yaml:"mem"` //内存
	vars variableMap //变量对象
}

//GetCPU 获得CPU配置
func (rc *ResourceConf) GetCPU() (float64, error) {
	cpu := rc.vars.var2Volumn(rc.CPU)
	return strconv.ParseFloat(cpu, 64)
}

//GetMem 获得内存配置
func (rc *ResourceConf) GetMem() (float64, error) {
	mem := rc.vars.var2Volumn(rc.Mem)
	return strconv.ParseFloat(mem, 64)
}

//PrintResConf 打印
func (rc *ResourceConf) PrintResConf() {
	fmt.Printf(" CPU: %s, MEM: %s\n", rc.CPU, rc.Mem)
}
