package conf

import "strings"

//variableMap 变量保存对象用来读写变量的接口
type variableMap interface {
	var2Volumn(v string) string
}

//Variables 变量定义
type Variables map[string]*string

func newVariable() Variables {
	return make(map[string]*string)
}

//将字符串中的变量转换为常量
//当前支持的变量为: $REGISTRY
func (vars *Variables) var2Volumn(v string) string {
	for k, value := range *vars {
		if !strings.HasPrefix(k, "$") {
			k = "$" + k
		}
		v = strings.Replace(v, k, *value, -1)
	}
	return v
}

//GetVariable 获得配置变量值
func (vars *Variables) getVariable(name string) *string {
	value, ok := (*vars)[name]
	if !ok {
		return nil
	}
	return value
}

//PutVariable 添加或更新一个配置变量
func (vars *Variables) putVariable(name string, value *string) {
	(*vars)[name] = value
}
