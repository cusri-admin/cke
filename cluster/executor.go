package cluster

import "sync"

//ExecutorInfo 存储任务使用的Executor的信息
type ExecutorInfo struct {
	Name  string //Executor(所代表的KubeNode)的名称，形如cluster.node
	ID    string //保存最终运行时的executor id,在Scheduler.createExecutor中赋值
	Image string
	Res   *Resource
	Cmd   string
	Args  []string
	Envs  map[string]string
}

//ExectorMap Executor列表
type ExectorMap struct {
	execs map[string]*ExecutorInfo
	lock  sync.RWMutex // 任务对象锁
}

//GetExec 获得指定主机上的Executor
func (em *ExectorMap) GetExec(host string) *ExecutorInfo {
	em.lock.RLock()
	defer em.lock.RUnlock()
	return em.execs[host]
}

//GetOrCreateExec 获得指定主机上的Executor
func (em *ExectorMap) GetOrCreateExec(host string) *ExecutorInfo {
	em.lock.RLock()
	defer em.lock.RUnlock()
	return em.execs[host]
}
