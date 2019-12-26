package conf

import (
	"bufio"
	"cke/log"
	"cke/utils"
	"io"
	"os"
	"strings"
)

//FetchDefaultConfig 从文件获取k8s集群的缺省配置
func FetchDefaultConfig(path string) (defaultConfig map[string]string, err error) {
	defaultConfig = make(map[string]string)
	filePath := utils.AppendPath(*defCfgPath, path)
	f, err := os.Open(filePath)
	if err != nil {
		log.Error("open file " + filePath + " error")
		return defaultConfig, err
	}
	defer f.Close()

	br := bufio.NewReader(f)
	var line, key string
	for {
		s, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}
		line = string(s)
		if 0 == len(line) || line == "\r\n" {
			continue
		}
		if strings.HasPrefix(line, "--") {
			key = line
			defaultConfig[key] = "#--#"
		} else {
			defaultConfig[key] = line
		}
	}
	return
}

//FetchDefaultConfigBytes 从存储中获取缺省的数据
func FetchDefaultConfigBytes(path string) ([]byte, error) {
	k8sDefaultCfgPath := utils.AppendPath(*defCfgPath, path)
	var defaultConfig []byte
	f, err := os.Open(k8sDefaultCfgPath)
	if err != nil {
		log.Error("open file " + path + " error")
		return nil, err
	}
	defer f.Close()

	byteBuffer := make([]byte, 1024)
	for {
		n, err := f.Read(byteBuffer)
		if err != nil && err != io.EOF {
			return nil, err
		}
		if n == 0 {
			break
		}
		defaultConfig = append(defaultConfig, byteBuffer[:n]...)
	}
	return defaultConfig, nil
}
