package utils

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

//CreateDir 创建目录
func CreateDir(dir string) (bool, error) {
	_, err := os.Stat(dir)
	if err == nil {
		//directory exists
		return true, nil
	}
	err2 := os.MkdirAll(dir, 0755)
	if err2 != nil {
		return false, err2
	}
	return true, nil
}

//RemoveDir 删除目录
func RemoveDir(dir string) (bool, error) {
	_, err := os.Stat(dir)
	if err != nil {
		//directory exists
		return false, nil
	}
	err2 := os.RemoveAll(dir)
	if err2 != nil {
		return false, err2
	}
	return true, nil
}

//RemoveParentDir 如果父目录为空，删除父目录
func RemoveParentDir(path string) (bool, error) {
	dir := filepath.Dir(path)
	dirs, _ := ioutil.ReadDir(dir)
	if len(dirs) <= 0 {
		return RemoveDir(dir)
	}
	return false, nil
}

//AppendPath 以path为根组装出subPaths为各级子目录的全路径
func AppendPath(path string, subPaths ...string) string {
	isSlash := func(r rune) bool {
		return r == '\\' || r == '/'
	}
	fullPath := strings.TrimRightFunc(path, isSlash)
	for _, subPath := range subPaths {
		fullPath += "/" + strings.TrimFunc(subPath, isSlash)
	}
	return fullPath
}
