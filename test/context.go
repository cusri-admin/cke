package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"sync"
	"time"
)

type triggerInst struct {
	wg  sync.WaitGroup
	exp *regexp.Regexp
	buf bytes.Buffer
}

//Context 测试上下文
type Context struct {
	cfg  *Config
	schs []*scheduler
	sync.Mutex
}

type scheNode struct {
	index int
	sch   *scheduler
}

func newContext(cfg *Config) (*Context, error) {
	ctx := &Context{
		cfg: cfg,
	}
	for i := 0; i < len(cfg.SchConf); i++ {
		sch, err := newScheduler(i, cfg.SchConf[i])
		if err != nil {
			return nil, err
		}
		ctx.schs = append(ctx.schs, sch)
	}
	return ctx, nil
}

//Start 启动测试
func (c *Context) Start() error {
	for i := 0; i < len(c.cfg.SchConf); i++ {
		if err := c.schs[i].start(); err != nil {
			c.Stop()
			return err
		}
	}
	//TODO waiting for started
	if err := c.WaitingOnce([]string{c.cfg.ReadyStr}, 10000); err != nil {
		return err
	}
	return nil
}

//StartOne 启动测试
func (c *Context) StartOne(index int) error {

	if err := c.schs[index].start(); err != nil {
		return err
	}

	//TODO waiting for started
	if err := c.WaitingOnce([]string{c.cfg.ReadyStr, "Waiting to be a leader"}, 10000); err != nil {
		return err
	}
	return nil
}

//Stop 停止测试
func (c *Context) Stop() {
	for _, sch := range c.schs {
		sch.stop()
	}
}

//StopOne 停止一个scheduler
func (c *Context) StopOne(index int) {
	c.schs[index].stop()
}

//GetClusterNameFromJSON 获得集群名
func (c *Context) GetClusterNameFromJSON(jsonFile string) (string, error) {
	//从文件的json中创建集群
	jsonData, err := ioutil.ReadFile(c.cfg.GetPath(jsonFile))
	if err != nil {
		return "", err
	}

	var dat map[string]interface{}
	if err := json.Unmarshal(jsonData, &dat); err != nil {
		return "", err
	}
	if v, ok := dat["name"]; ok {
		if value, ok := v.(string); ok {
			return value, nil
		}
		return "", fmt.Errorf("cluster name is not string in json file: %s", jsonFile)
	}
	return "", fmt.Errorf("Can't find cluster name in json file: %s", jsonFile)
}

//GetNodeNamesFromJSON 获得集群名
func (c *Context) GetNodeNamesFromJSON(jsonFile string) ([]string, error) {
	jsonData, err := ioutil.ReadFile(c.cfg.GetPath(jsonFile))
	if err != nil {
		return nil, err
	}

	var dat []map[string]interface{}
	if err := json.Unmarshal(jsonData, &dat); err != nil {
		return nil, err
	}

	names := make([]string, 0)
	for _, node := range dat {
		if v, ok := node["name"]; ok {
			if value, ok := v.(string); ok {
				names = append(names, value)
			} else {
				return nil, fmt.Errorf("node name is not string in json file: %s", jsonFile)
			}
		} else {
			return nil, fmt.Errorf("Can't find node name in json file: %s", jsonFile)
		}
	}
	return names, nil
}

func (c *Context) getURLs(url string) []string {
	urls := make([]string, 0)
	for _, sch := range c.schs {
		urls = append(urls, sch.cfg.getURL(url))
	}
	return urls
}

//CreateCluster 创建集群
func (c *Context) CreateCluster(jsonFile string, timeOut int64) error {
	//从文件的json中创建集群
	if err := PostJSONFile2CKE(c.getURLs("/clusters"), c.cfg.GetPath(jsonFile)); err != nil {
		return err
	}

	//等待创建成功
	if err := c.WaitingOnce([]string{c.cfg.OKStr}, timeOut); err != nil {
		return err
	}
	return nil
}

//DeleteCluster 删除集群
func (c *Context) DeleteCluster(name string, timeOut int64) error {
	//删除集群
	if err := DeleteCKE(c.getURLs("/clusters/" + name)); err != nil {
		return err
	}

	//等待删除成功
	if err := c.WaitingOnce([]string{c.ReadyString()}, timeOut); err != nil {
		return err
	}
	return nil
}

//WaitingAll 等待所有事件发生
// exp 事件(日志)发生的匹配的exp
// timeOut 超时时间(毫秒)
func (c *Context) WaitingAll(exps []string, timeOut int64) error {
	lock := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	c.Lock()
	chs := make([]chan []byte, 0)
	for _, exp := range exps {
		wg.Add(1)
		for _, sch := range c.schs {
			ch := make(chan []byte, 1024)
			chs = append(chs, ch)
			go func(lock *sync.Mutex, exp string, wg **sync.WaitGroup, sch *scheduler) {
				sch.waiting(exp, ch)
				lock.Lock()
				defer lock.Unlock()
				if (*wg) != nil {
					(*wg).Add(-1)
				}
			}(lock, exp, &wg, sch)
		}
	}
	c.Unlock()

	flag := false
	if timeOut > 0 {
		timer := time.AfterFunc(time.Duration(timeOut)*time.Millisecond, func() {
			flag = true
			wg.Done()
		})
		defer timer.Stop()
	}

	wg.Wait()
	lock.Lock()
	wg = nil
	lock.Unlock()

	for _, ch := range chs {
		close(ch)
	}

	if flag {
		return fmt.Errorf("time out")
	}
	return nil
}

//WaitingOnce 等待一个事件发生
// exp 事件(日志)发生的匹配的exp
// timeOut 超时时间(毫秒)
func (c *Context) WaitingOnce(exps []string, timeOut int64) error {
	lock := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	wg.Add(1)
	c.Lock()
	chs := make([]chan []byte, 0)
	for _, exp := range exps {
		for _, sch := range c.schs {
			ch := make(chan []byte, 1024)
			chs = append(chs, ch)
			go func(lock *sync.Mutex, exp string, wg **sync.WaitGroup, sch *scheduler) {
				sch.waiting(exp, ch)
				lock.Lock()
				defer lock.Unlock()
				if (*wg) != nil {
					(*wg).Add(-1)
				}
			}(lock, exp, &wg, sch)
		}
	}
	c.Unlock()

	flag := false
	if timeOut > 0 {
		timer := time.AfterFunc(time.Duration(timeOut)*time.Millisecond, func() {
			flag = true
			wg.Done()
		})
		defer timer.Stop()
	}

	wg.Wait()
	lock.Lock()
	wg = nil
	lock.Unlock()

	for _, ch := range chs {
		close(ch)
	}

	if flag {
		return fmt.Errorf("time out")
	}
	return nil
}

//DefaultTimeOut 获得缺省时延
func (c *Context) DefaultTimeOut() int64 {
	return c.cfg.DefTimeOut
}

//NodeOKString 条件数据
func (c *Context) NodeOKString(name string) string {
	return fmt.Sprintf(c.cfg.NodeOKStr, name)
}

//ReadyString 条件数据
func (c *Context) ReadyString() string {
	return c.cfg.ReadyStr
}

//PostJSONFile2CKE 向CKE post指定的json文件
func (c *Context) PostJSONFile2CKE(url string, jsonFile string) error {
	//从文件的json中创建集群
	return PostJSONFile2CKE(c.getURLs(url), c.cfg.GetPath(jsonFile))
}

//DeleteMethod 删除集群
func (c *Context) DeleteMethod(url string, timeOut int64) error {
	//删除集群
	if err := DeleteCKE(c.getURLs(url)); err != nil {
		return err
	}
	return nil
}
