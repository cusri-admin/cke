package config

import (
    "os"
    "strings"
)

/*****************
  Author: Chen
  Date: 2019/3/14
  Comp: ChinaUnicom
 ****************/

/**
 *  executor运行参数配置
 * 主要包括：
 * 存储配置参数
 * 网络配置参数
 * 镜像环境
 * 其他外围参数
 */

var (
    CephMonitors string
)

type Config struct {
    Network map[string]interface{}
    Storage map[string]interface{}
    Compute map[string]interface{}
}

func (c *Config) Init(regionId string) {
    CephMonitors, isExist := os.LookupEnv("CEPH_MONITORS")
    if !isExist {
        CephMonitors = "10.124.142.59:6789"
    }
    c.Storage["cephMonitor"] = strings.Split(CephMonitors, ",")
    c.Storage["regionId"] = regionId
}
