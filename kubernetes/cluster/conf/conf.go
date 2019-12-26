package conf

import (
	"cke"
	"cke/log"
	"cke/utils"
	"flag"
	"io/ioutil"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
)

//启动(初始化)期
//	参数由三级赋值组成
//	低级：程序内配置、中级：环境变量、高级：命令行。高级别的设置会替代低级别的配置
//运行后，每次参数使用时会对参数中的变量进行实际值替换
//已知问题：请注意循环替换的问题，既 A包含B，B包含A

//指定配置文件路径的参数
var (
	//defCfgPath = flag.String("k8s_config", "./default_cfg/cke.conf.yaml", "The path of Kubernetes config file.")
	defCfgPath = flag.String("k8s_config", "./default_cfg/", "The path of Kubernetes config file.")
	imagePath  = flag.String("k8s_image_path", "reg.mg.hcbss/open", "CKE KubeNode docker image path")
)

//Parameter 表示配置文件中的参数，
//可以指定是否允许用户修改
type Parameter struct {
	Key         string `yaml:"key"`    //参数名称
	Value       string `yaml:"value"`  //参数值
	AllowModify bool   `yaml:"modify"` //是否允许用户修改
}

//setParameter 更新[]Parameter中的一个Parameter的设置
//如果当前配置中允许更新的话
//如果当前配置中没有找到对应的项，则添加一项
func setParameter(key, value string, paras *[]*Parameter) {
	for _, para := range *paras {
		if para.Key == key {
			if para.AllowModify {
				para.Value = value
			}
			return
		}
	}
	*paras = append(*paras, &Parameter{
		Key:         key,
		Value:       value,
		AllowModify: true,
	})

	log.Debugf("setParameter key %s value: %s", key, value)
}

func getParameter(key string, paras *[]*Parameter) string {
	for _, para := range *paras {
		if para.Key == key {
			return para.Value
		}
	}
	return ""
}

func getImagePath(name string) string {
	if len(*imagePath) > 0 {
		if strings.HasSuffix(*imagePath, "/") {
			return *imagePath + name
		}
		return *imagePath + "/" + name
	}
	return name
}

//LoadCKEConf 从指定的路径下读取对应的进程配置信息
//LoadCKEConf 从指定的路径下读取对应的进程配置信息
func LoadCKEConf(ver string) (*CKEConf, error) {
	var fileName string
	if len(ver) <= 0 {
		fileName = "cke.conf.yaml"
	} else {
		fileName = "cke.conf-" + ver + ".yaml"
	}
	filePath := utils.AppendPath(*defCfgPath, fileName)
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Errorf("Read process config file %s error: %s", filePath, err.Error())
		return nil, err
	}
	ckeConf := &CKEConf{
		Variables: newVariable(),
	}
	if err := yaml.Unmarshal(data, ckeConf); err != nil {
		log.Errorf("Unmarshal process config file %s error: %s", filePath, err.Error())
		return nil, err
	}
	//添加版本信息
	ckeConf.PutVariable("VERSION", &cke.VERSION)

	return ckeConf, nil
}

//SupportedVersions 通过分析配置文件名返回所有支持的K8S版本
func SupportedVersions() []string {
	vers := make([]string, 0)
	files, err := ioutil.ReadDir(*defCfgPath)
	if err != nil {
		log.Errorf("Read path %s error: %s", *defCfgPath, err.Error())
		return vers
	}
	for _, oneFile := range files {
		if !oneFile.IsDir() {
			//正则匹配型如"cke.conf-x.xx.yaml"的文件名，并提取x.xx
			re := regexp.MustCompile(`^cke.conf-([\d]+.[\d]+).yaml$`)
			matched := re.FindAllStringSubmatch(oneFile.Name(), -1)
			if len(matched) == 1 {
				vers = append(vers, matched[0][1])
			}
		}
	}
	return vers
}
