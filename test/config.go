package test

import (
	"fmt"
	"io/ioutil"
	"strings"

	"gopkg.in/yaml.v2"
)

//SchConf scheduler 启动配置
type SchConf struct {
	Path    string   `yaml:"path"`
	Args    []string `yaml:"args"`
	BaseURL string   `yaml:"base_url"`
}

//GetURL 获得全路径
func (sc *SchConf) getURL(path string) string {
	url := sc.BaseURL
	if strings.HasSuffix(url, "/") {
		url = sc.BaseURL[:len(url)-1]
	}
	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}
	return url + "/" + path
}

//Config 测试环境配置
type Config struct {
	SchConf    []*SchConf `yaml:"schedulers"`
	BasePath   string     `yaml:"base_path"`
	ReadyStr   string     `yaml:"ready_str"`
	OKStr      string     `yaml:"ok_str"`
	DefTimeOut int64      `yaml:"default_time_out"`
	NodeOKStr  string     `yaml:"node_ok_str"`
}

//LoadConfig 从文件读取测试环境配置
func LoadConfig(filePath string, name string) (*Config, error) {

	var fileName string
	if strings.HasSuffix(filePath, "/") {
		fileName = filePath + name + ".yaml"
	} else {
		fileName = filePath + "/" + name + ".yaml"
	}

	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("read config file %s error: %s", fileName, err.Error())
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("unmarshal process config file %s error: %s", fileName, err.Error())
	}

	return cfg, nil
}

//GetPath 获得全路径
func (c *Config) GetPath(path string) string {
	basePath := c.BasePath
	if strings.HasSuffix(basePath, "/") {
		basePath = basePath[:len(basePath)-1]
	}
	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}
	return basePath + "/" + path
}
