package framework

import (
	cc "cke/cluster"
	"cke/log"
	"cke/utils"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	mesos "github.com/mesos/mesos-go/api/v1/lib"
	scheduler "github.com/mesos/mesos-go/api/v1/lib/scheduler"
)

const offsetFloat = 0.0000001 //纠正浮点比较的不正确性

//Scheduler 描述mesos的scheduler
type Scheduler struct {
	roles           []string
	fw              *Framework
	clusterManager  *cc.ClusterManager
	isRunning       bool
	saveFwIDFunc    func(string)
	executorURL     string
	mesosVersion    string
	enableOfferLock sync.Mutex
	enableOffer     bool //是否允许接收Offer
}

//Initialize 初始化framework
func Initialize(frameworkName string,
	role *string,
	masters []string,
	frameworkID *string,
	saveFwIDFunc func(string),
	clusterManager *cc.ClusterManager,
	executorURL string,
	webURL string,
	failover int64,
	delayFailure int64,
	retryCount int64) *Scheduler {

	roles := strings.Split(*role, ",")

	sched := &Scheduler{
		roles:          roles,
		clusterManager: clusterManager,
		isRunning:      true,
		fw:             newFramework(frameworkName, roles, masters, frameworkID, webURL, failover, delayFailure, retryCount),
		saveFwIDFunc:   saveFwIDFunc,
		executorURL:    executorURL,
		enableOffer:    true,
	}
	sched.fw.receiver = sched.receive
	clusterManager.Initialize(sched)
	return sched
}

func getTaskMesosStatus(mesosState *mesos.TaskState) (state cc.TaskMesosState) {
	switch *mesosState {
	case mesos.TASK_RUNNING:
		return cc.TASKMESOS_RUNNING
	case mesos.TASK_KILLING:
		return cc.TASKMESOS_KILLING
	case mesos.TASK_FINISHED:
		return cc.TASKMESOS_FINISHED
	case mesos.TASK_FAILED:
		return cc.TASKMESOS_FAILED
	case mesos.TASK_KILLED:
		return cc.TASKMESOS_KILLED
	case mesos.TASK_LOST:
		return cc.TASKMESOS_LOST
	case mesos.TASK_DROPPED:
		return cc.TASKMESOS_FAILED
	}
	return cc.TASKMESOS_STARTING
}

func convResource(res *mesos.Resource) string {
	providerID := ""
	if res.ProviderID != nil {
		providerID = res.ProviderID.Value
	}
	resString := fmt.Sprintf("ProviderID: %s, %s", providerID, res.Name)
	switch *res.Type {
	case mesos.SCALAR:
		resString += fmt.Sprintf(" %f", res.Scalar.Value)
		break
	case mesos.RANGES:
		resString += fmt.Sprintf(" %v", res.Ranges.Range)
		break
	case mesos.SET:
		resString += fmt.Sprintf(" %v", res.Set.Item)
		break
	default:
		resString = " ???"
	}
	return resString + " " + convReservation(res.Reservation)
}

func convReservation(r *mesos.Resource_ReservationInfo) string {
	if r != nil {
		labels := ""
		if r.Labels != nil {
			for _, lab := range r.Labels.Labels {
				labels += lab.Key + ":" + *lab.Value + ","
			}
		}
		role := ""
		if r.Role != nil {
			role = *r.Role
		}
		return fmt.Sprintf("[Role(%s), labels:%s]", role, labels)
	}
	return ""
}

func convOffer(offer *mesos.Offer) string {
	offStr := fmt.Sprintf("Offer: %s\n  Hostname: %s, Agent: %s\n",
		offer.ID, offer.Hostname, offer.AgentID)
	for i := 0; i < len(offer.Resources); i++ {
		res := offer.Resources[i]
		offStr += fmt.Sprintf("    Role: %s, Res: %s\n", *res.Role, convResource(&res))
	}
	return offStr
}

func printOffer(offer *mesos.Offer) {
	log.Debug(convOffer(offer))
}

func printOffers(offers []mesos.Offer) {
	offersStr := ""
	for _, offer := range offers {
		offersStr += convOffer(&offer)
	}
	log.Debug(offersStr)
}

func createContainer(image string) (container *mesos.ContainerInfo) {
	containerType := mesos.ContainerInfo_DOCKER
	network := mesos.ContainerInfo_DockerInfo_HOST
	return &mesos.ContainerInfo{
		Type: &containerType,
		Docker: &mesos.ContainerInfo_DockerInfo{
			Image:   image,
			Network: &network,
		},
	}
}

func (sch *Scheduler) createExecEnvironment(exec *cc.ExecutorInfo) (execEnvs *mesos.Environment) {
	envsCount := len(exec.Envs)
	if envsCount > 0 {
		envVars := make([]mesos.Environment_Variable, 0, envsCount+1)
		//log flag
		logLevel := log.GetLevel().String()
		envVars = append(envVars, mesos.Environment_Variable{
			Type:  mesos.Environment_Variable_VALUE.Enum(),
			Name:  log.ENV_LOG_LEVEL,
			Value: &logLevel,
		})
		for k, v := range exec.Envs {
			envVars = append(envVars, mesos.Environment_Variable{
				Type:  mesos.Environment_Variable_VALUE.Enum(),
				Name:  k,
				Value: &v,
			})
		}
		execEnvs = &mesos.Environment{
			Variables: envVars,
		}
	}
	return
}

func (sch *Scheduler) createExecutor(exec *cc.ExecutorInfo, resources []mesos.Resource) *mesos.ExecutorInfo {
	hasShell := false
	exec.ID = fmt.Sprintf("CKE.%s.%s", utils.HashString(sch.fw.FrameworkID.Value), exec.Name)
	executable := true
	cmdPath := "./" + exec.Cmd
	return &mesos.ExecutorInfo{
		Type: mesos.ExecutorInfo_CUSTOM,
		ExecutorID: mesos.ExecutorID{
			Value: exec.ID,
		},
		FrameworkID: sch.fw.FrameworkID,
		Command: &mesos.CommandInfo{
			URIs: []mesos.CommandInfo_URI{
				mesos.CommandInfo_URI{
					Value:      sch.executorURL + "exec/" + exec.Cmd,
					Executable: &executable,
					// Extract:    true,
					// Cache:      true,
					// OutputFile: "",
				},
			},
			Environment: sch.createExecEnvironment(exec),
			Shell:       &hasShell,
			Value:       &cmdPath,
			Arguments:   exec.Args,
			//User:      &user,
		},
		Resources: resources,
		//Container: createContainer("cke-exec-k8s:0.0.1"),
	}
}

func splitScalarResource(value float64, resource *mesos.Resource) mesos.Resource {
	resource.Scalar.Value -= value
	return mesos.Resource{
		ProviderID:   resource.ProviderID,
		Name:         resource.Name,
		Type:         resource.Type,
		Scalar:       &mesos.Value_Scalar{Value: value},
		Role:         resource.Role,
		Reservation:  resource.Reservation,
		Reservations: resource.Reservations,
	}
}

//isAvailable 判断指定的task是否可以使用指定的资源。
//条件是该资源是事先预留的(具有特殊的label)
func (sch *Scheduler) isAvailable(task *cc.TaskInfo, resource *mesos.Resource) bool {
	if task.Reservation == nil {
		return true
	}
	if resource.Reservation != nil {
		key, id := sch.getReservationLabel(task.Reservation)
		labels := resource.Reservation.Labels
		if task.Reservation.PreReserved {
			//TODO: 这个只是适配了西咸的DC/OS模式，待改进
			if resource.Reservation.Labels != nil {
				for _, lab := range labels.Labels {
					if lab.Key == key {
						return false
					}
				}
			}
			return true
		}
		if resource.Reservation.Labels != nil {
			for _, lab := range labels.Labels {
				if lab.Key == key && *lab.Value == id {
					return true
				}
			}
		}
	}
	return false
}

func (sch *Scheduler) buildLaunchOperation(offer *mesos.Offer) []*mesos.Offer_Operation {
	finished, taskMap := sch.clusterManager.GetInconsistentTasksOfClusters()
	//没有任务，不用生成 mesos.Offer_Operation
	if len(taskMap) <= 0 {
		if finished {
			if err := sch.stopOffer(); err != nil {
				log.Errorf("Supress error: %s", err.Error())
			}
		}
		return nil
	}
	var cpusOfferRes *mesos.Resource
	var memOfferRes *mesos.Resource
	var exec *mesos.ExecutorInfo
	mesosTasks := make([]mesos.TaskInfo, 0, 10)
	for cluster, tasks := range taskMap {
		for _, task := range tasks {
			//针对每个Task找到合适的资源
			if strings.Compare(*task.Host, offer.Hostname) == 0 ||
				len(*task.Host) <= 0 ||
				*task.Host == "*" {
				//使用指针保存符合条件的资源，即便后续做核减(减少已使用的量)
				cpusOfferRes = nil
				memOfferRes = nil
				for n := range offer.Resources {
					res := &offer.Resources[n]
					if sch.isAvailable(task, res) {
						if cpusOfferRes == nil && res.Name == "cpus" {
							if res.Scalar.Value+offsetFloat >= task.Executor.Res.CPU {
								cpusOfferRes = res
							}
						} else if memOfferRes == nil && res.Name == "mem" {
							if res.Scalar.Value+offsetFloat >= task.Executor.Res.Mem {
								memOfferRes = res
							}
						}
					}
				}
				if cpusOfferRes != nil && memOfferRes != nil {
					//生成二进制的task数据
					bodyData, err := cluster.MarshalTask(task.Body)
					if err != nil {
						log.Errorf("Launch task %s error: %s", task.Name, err.Error())
					} else {
						//创建 task对应的executor
						//计算executor使用的资源，同时将目标资源(cpusOfferRes\memOfferRes)做相应的减少
						cpusRes := splitScalarResource(task.Executor.Res.CPU, cpusOfferRes)
						memRes := splitScalarResource(task.Executor.Res.Mem, memOfferRes)
						exec = sch.createExecutor(task.Executor, []mesos.Resource{cpusRes, memRes})

						if cpusOfferRes.Scalar.Value+offsetFloat >= task.Res.CPU &&
							memOfferRes.Scalar.Value+offsetFloat >= task.Res.Mem {

							//及时更新task的实际落地启动的Host地址，通过指针直接改变所有相关task约束
							*task.Host = offer.Hostname
							cpusRes := splitScalarResource(task.Res.CPU, cpusOfferRes)
							memRes := splitScalarResource(task.Res.Mem, memOfferRes)
							//任务名，对应的mesos task的TaskId为该字段加上"."再加上一个16进制表示的64bit的随机数
							//任务名在framework中不能重复，唯一表示一个进程或虚拟机
							taskName := cluster.GetName() + "." + task.Name
							taskID := fmt.Sprintf("%s.%08X", taskName, rand.Int31())
							t := mesos.TaskInfo{
								Name: taskName,
								TaskID: mesos.TaskID{
									Value: taskID,
								},
								AgentID:   offer.AgentID,
								Resources: []mesos.Resource{cpusRes, memRes},
								Executor:  exec,
								Data:      bodyData,
							}
							mesosTasks = append(mesosTasks, t)

							//将任务的状态更新到已部署但未运行的状态
							status := &cc.TaskStatus{
								Id:      taskID,
								AgentId: offer.AgentID.Value,
								State:   cc.TASKMESOS_STAGING,
							}
							sch.clusterManager.UpdateTaskStatus(status)
						}
					}
				}
			}
		}
	}
	if len(mesosTasks) > 0 {
		return []*mesos.Offer_Operation{
			&mesos.Offer_Operation{
				Type: mesos.Offer_Operation_LAUNCH,
				Launch: &mesos.Offer_Operation_Launch{
					TaskInfos: mesosTasks,
				},
			},
		}
	}
	return nil
}

//getReservationLabel 生成资源预留中的标签
func (sch *Scheduler) getReservationLabel(r *cc.Reservation) (string, string) {
	return "CKE-ID", fmt.Sprintf("%s.%s", sch.fw.FrameworkID.Value, r.ReservationID)
}

/*
预留算法说明
前提
  1)针对一个主机的资源Offer，一定优先获得Reseved的资源
  2)scheduler的各种操作是串行的
算法
  当前使用量+offer=Reserved
  如果reserved小于target则需要对资源进行预留，预留量=target-Reserved
  如果reserved大于target则需要对资源进行释放，释放量=Reserved-target
流程
  获得使用量
  计算当前offer中的资源量
  和target比较，生成预留/释放操作
*/
//buildHostResOper 生成指定主机上的预留资源
func (sch *Scheduler) buildHostResOper(r *cc.Reservation, offer *mesos.Offer) ([]mesos.Resource, []mesos.Resource) {
	//获得当前使用量
	usedRes, ok := r.GetUsedResource()
	if !ok {
		return nil, nil
	}

	//找到已预留的资源,判断标准为Reservation中的label符合要求
	cpusOfferRes := 0.0
	memOfferRes := 0.0
	//quota中可用资源(*资源)
	starCPU := 0.0
	starMem := 0.0
	key, id := sch.getReservationLabel(r)
	for n := range offer.Resources {
		res := &offer.Resources[n]
		if res.Role != nil && *(res.Role) == sch.roles[0] {
			if res.Reservation != nil && res.Reservation.Labels != nil {
				for _, lab := range res.Reservation.Labels.Labels {
					if lab.Key == key && lab.Value != nil && *lab.Value == id {
						if res.Name == "cpus" {
							cpusOfferRes = res.Scalar.Value
							break
						} else if res.Name == "mem" {
							memOfferRes = res.Scalar.Value
							break
						}
					}
				}
			}
		} else if res.Role != nil && *(res.Role) == "*" {
			if res.Name == "cpus" {
				starCPU = res.Scalar.Value
			} else if res.Name == "mem" {
				starMem = res.Scalar.Value
			}
		}
	}

	//是否所有当前(r.Current)资源已经上报和使用(各任务处在稳定状态)
	gotAllCPU := math.Abs(r.Current.CPU-cpusOfferRes-usedRes.CPU) < offsetFloat
	gotAllMem := math.Abs(r.Current.Mem-memOfferRes-usedRes.Mem) < offsetFloat

	//是否所有期望(r.Target)的资源已经上报和使用
	successful := math.Abs(r.Target.Mem-memOfferRes-usedRes.Mem) < offsetFloat &&
		math.Abs(r.Target.CPU-cpusOfferRes-usedRes.CPU) < offsetFloat

	log.Debugf("ID: %s\tHost: %s\n"+
		"\tTarget.CPU: %0.2f\tTarget.Mem: %0.2f\n"+
		"\tOfferCPU: %0.2f\tOfferMem: %0.2f\n"+
		"\tUsedCPU: %0.2f\tUsedMem: %0.2f\n"+
		"\tCurrentCPU: %0.2f\tCurrentMem: %0.2f\n"+
		"\tgotAllCPU: %t\tgotAllMem: %t\n"+
		"\tStarCPU: %0.2f\tStarMem: %0.2f\n"+
		"\tsuccessful: %t",
		r.ReservationID, offer.Hostname,
		r.Target.CPU, r.Target.Mem,
		cpusOfferRes, memOfferRes,
		usedRes.CPU, usedRes.Mem,
		r.Current.CPU, r.Current.Mem,
		gotAllCPU, gotAllMem,
		starCPU, starMem,
		successful)

	if successful {
		log.Infof("Operate resource seccessfully %s: CurrentCPU: %0.2f, CurrentMem: %0.2f, Target.CPU: %0.2f, Target.Mem: %0.2f at %s",
			r.ReservationID, r.Current.CPU, r.Current.Mem, r.Target.CPU, r.Target.Mem, offer.Hostname)
		r.Current.Set(r.Target)
		r.IsNeedOperation = false
		//完成回调
		if r.Callback != nil {
			r.Callback()
		}
	}

	//判断*资源是否够预留的
	//在释放资源时r.Target.CPU-r.Current.CPU会得到负值，资源为0同样满足
	sufficientStarRes := (r.Target.CPU-r.Current.CPU) < starCPU && (r.Target.Mem-r.Current.Mem) < starMem

	//OperationID := fmt.Sprintf("%s-%08X", r.ReservationID, rand.Int31())
	principal := "CKE"
	reserveRes := []mesos.Resource{}
	unreserveRes := []mesos.Resource{}

	if gotAllCPU { //判断是否是所有资源已经上报，防止executor等泄漏资源
		if math.Abs(cpusOfferRes+usedRes.CPU-r.Target.CPU) > offsetFloat {
			if cpusOfferRes+usedRes.CPU < r.Target.CPU && sufficientStarRes {
				//需要预留cpu
				//判断*资源是否够预留的

				*r.Host = offer.Hostname
				r.AgentID = offer.AgentID.Value
				cpus := r.Target.CPU - r.Current.CPU
				reserveRes = append(reserveRes, mesos.Resource{
					//ProviderID: &mesos.ResourceProviderID{Value: offer.AgentID.Value},
					Name:   "cpus",
					Type:   mesos.SCALAR.Enum(),
					Scalar: &mesos.Value_Scalar{Value: cpus},
					Reservations: []mesos.Resource_ReservationInfo{
						mesos.Resource_ReservationInfo{
							Type:      mesos.Resource_ReservationInfo_DYNAMIC.Enum(),
							Role:      &sch.roles[0],
							Principal: &principal,
							Labels: &mesos.Labels{
								Labels: []mesos.Label{
									mesos.Label{
										Key:   key,
										Value: &id,
									},
								},
							},
						},
					},
				})
				log.Infof("Reserving CPU: %0.2f, ID: %s, OfferCPU: %0.2f, UsedCPU: %0.2f, Target.CPU: %0.2f at %s",
					cpus, r.ReservationID, cpusOfferRes, usedRes.CPU, r.Target.CPU, offer.Hostname)
			} else if cpusOfferRes+usedRes.CPU > r.Target.CPU {
				//需要解预留
				cpus := cpusOfferRes + usedRes.CPU - r.Target.CPU
				unreserveRes = append(reserveRes, mesos.Resource{
					//ProviderID: &mesos.ResourceProviderID{Value: offer.AgentID.Value},
					Name:   "cpus",
					Type:   mesos.SCALAR.Enum(),
					Scalar: &mesos.Value_Scalar{Value: cpus},
					Reservations: []mesos.Resource_ReservationInfo{
						mesos.Resource_ReservationInfo{
							Type:      mesos.Resource_ReservationInfo_DYNAMIC.Enum(),
							Role:      &sch.roles[0],
							Principal: &principal,
							Labels: &mesos.Labels{
								Labels: []mesos.Label{
									mesos.Label{
										Key:   key,
										Value: &id,
									},
								},
							},
						},
					},
				})
				log.Infof("Unreserving CPU: %0.2f, ID: %s, OfferCPU: %0.2f, UsedCPU: %0.2f, Target.CPU: %0.2f at %s",
					cpus, r.ReservationID, cpusOfferRes, usedRes.CPU, r.Target.CPU, offer.Hostname)
			}
		}
	}

	if gotAllMem { //判断是否是所有资源已经上报，防止executor等泄漏资源
		if math.Abs(memOfferRes+usedRes.Mem-r.Target.Mem) > offsetFloat {
			if memOfferRes+usedRes.Mem < r.Target.Mem && sufficientStarRes {
				//需要预留Mem
				//判断*资源是否够预留的
				*r.Host = offer.Hostname
				r.AgentID = offer.AgentID.Value
				mem := r.Target.Mem - memOfferRes - usedRes.Mem
				reserveRes = append(reserveRes, mesos.Resource{
					//ProviderID: &mesos.ResourceProviderID{Value: offer.AgentID.Value},
					Name:   "mem",
					Type:   mesos.SCALAR.Enum(),
					Scalar: &mesos.Value_Scalar{Value: mem},
					Reservations: []mesos.Resource_ReservationInfo{
						mesos.Resource_ReservationInfo{
							Type:      mesos.Resource_ReservationInfo_DYNAMIC.Enum(),
							Role:      &sch.roles[0],
							Principal: &principal,
							Labels: &mesos.Labels{
								Labels: []mesos.Label{
									mesos.Label{
										Key:   key,
										Value: &id,
									},
								},
							},
						},
					},
				})
				log.Infof("Reserving Mem: %0.2f, ID: %s, OfferMem: %0.2f, UsedMem: %0.2f, Target.Mem: %0.2f at %s",
					mem, r.ReservationID, memOfferRes, usedRes.Mem, r.Target.Mem, offer.Hostname)
			} else if memOfferRes+usedRes.Mem > r.Target.Mem {
				//需要解预留
				mem := memOfferRes + usedRes.Mem - r.Target.Mem
				unreserveRes = append(reserveRes, mesos.Resource{
					//ProviderID: &mesos.ResourceProviderID{Value: offer.AgentID.Value},
					Name:   "mem",
					Type:   mesos.SCALAR.Enum(),
					Scalar: &mesos.Value_Scalar{Value: mem},
					Reservations: []mesos.Resource_ReservationInfo{
						mesos.Resource_ReservationInfo{
							Type:      mesos.Resource_ReservationInfo_DYNAMIC.Enum(),
							Role:      &sch.roles[0],
							Principal: &principal,
							Labels: &mesos.Labels{
								Labels: []mesos.Label{
									mesos.Label{
										Key:   key,
										Value: &id,
									},
								},
							},
						},
					},
				})
				log.Infof("Uneserving Mem: %0.2f, ID: %s, OfferMem: %0.2f, UsedMem: %0.2f, Target.Mem: %0.2f at %s",
					mem, r.ReservationID, memOfferRes, usedRes.Mem, r.Target.Mem, offer.Hostname)
			}
		}
	}

	//reserve和unreserve不能同时进行
	if len(reserveRes) > 0 {
		return reserveRes, nil
	}
	if len(unreserveRes) > 0 {
		return nil, unreserveRes
	}
	return nil, nil
}

//buildResourceOperation 根据每个cluster返回的数据进行预留/释放操作对象的生成
func (sch *Scheduler) buildResourceOperation(offer *mesos.Offer) []*mesos.Offer_Operation {
	//TODO: 对应于多Role的场景，当前的预留只是选择第一个Role是不正确的
	rs := sch.clusterManager.GetReservationOfClusters()
	if rs == nil {
		return nil
	}

	//判断每一个预留对象否操作
	for _, r := range rs {
		//首先判断主机匹配而且需要操作才需要处理
		if *r.Host == offer.Hostname || *r.Host == "*" || len(*r.Host) <= 0 {
			//尝试生成该主机的资源操作
			reserveRes, unreserveRes := sch.buildHostResOper(r, offer)
			opers := make([]*mesos.Offer_Operation, 0)
			if len(reserveRes) > 0 {
				reserveOper := &mesos.Offer_Operation{
					//ID:   &mesos.OperationID{Value: OperationID},
					Type:    mesos.Offer_Operation_RESERVE,
					Reserve: &mesos.Offer_Operation_Reserve{},
				}
				reserveOper.Reserve.Resources = append(reserveOper.Reserve.Resources, reserveRes...)
				opers = append(opers, reserveOper)
			}
			if len(unreserveRes) > 0 {
				unreserveOper := &mesos.Offer_Operation{
					//ID:   &mesos.OperationID{Value: OperationID},
					Type:      mesos.Offer_Operation_UNRESERVE,
					Unreserve: &mesos.Offer_Operation_Unreserve{},
				}
				unreserveOper.Unreserve.Resources = append(unreserveOper.Unreserve.Resources, unreserveRes...)
				opers = append(opers, unreserveOper)
			}
			if len(opers) > 0 {
				return opers
			}
		}
	}
	return nil
}

//processOffer 判断一个offer中的资源是否需要进行预留
func (sch *Scheduler) processOffer(offer *mesos.Offer) []*mesos.Offer_Operation {
	//判读一下是否需要预留/释放该offer的资源
	opers := sch.buildResourceOperation(offer)
	if opers != nil {
		return opers
	}

	//该offer没有被预留则生成需要执行的 Task
	return sch.buildLaunchOperation(offer)
}

//因为Accept的Operation只能针对一个Agent，所以需要对生成的OPeration进行分类
//该结构保存offer和operation的关联关系
type accept struct {
	OfferID mesos.OfferID
	oper    *mesos.Offer_Operation
}

func (sch *Scheduler) processOffers(offers []mesos.Offer) (accepts map[string][]*accept, declineIds []mesos.OfferID) {
	if log.DEBUG >= log.GetLevel() {
		printOffers(offers)
	}
	accepts = make(map[string][]*accept)
	declineIds = make([]mesos.OfferID, 0, len(offers))
	for _, offer := range offers {
		opers := sch.processOffer(&offer)
		if len(opers) > 0 {
			for _, oper := range opers {
				//accepts中保存按agent分类的operation
				if accepts[offer.AgentID.Value] == nil {
					accepts[offer.AgentID.Value] = make([]*accept, 0)
				}
				accepts[offer.AgentID.Value] = append(accepts[offer.AgentID.Value], &accept{
					OfferID: offer.ID,
					oper:    oper,
				})
			}
		} else {
			declineIds = append(declineIds, offer.ID)
		}
	}
	return
}

func (sch *Scheduler) onOffers(offersEvent *scheduler.Event_Offers) {
	//TODO 判断是否所有task都有当前运行状态，否则啥也不做
	accepts, declineIds := sch.processOffers(offersEvent.Offers)
	if len(accepts) > 0 {
		refuseSeconds := 0.01
		for _, aps := range accepts {
			toAcceptIds := make([]mesos.OfferID, 0)
			toOpers := make([]mesos.Offer_Operation, 0)
			for _, ap := range aps {
				toAcceptIds = append(toAcceptIds, ap.OfferID)
				toOpers = append(toOpers, *ap.oper)
			}
			accept := &scheduler.Call_Accept{
				OfferIDs:   toAcceptIds,
				Operations: toOpers,
				Filters: &mesos.Filters{
					RefuseSeconds: &refuseSeconds,
				},
			}
			// if log.GetLevel() >= log.DEBUG {
			// 	data, err := json.Marshal(accept)
			// 	if err != nil {
			// 		log.Errorf("Error:%s", err.Error())
			// 	} else {
			// 		log.Errorf("Accept: %s", string(data))
			// 	}
			// }
			err := sch.fw.CallMaster(accept)
			if err != nil {
				log.Error("Call ACCEPT error:", err)
			}
		}
	}

	if len(declineIds) > 0 {
		declineSeconds := 10.0
		decline := &scheduler.Call_Decline{
			OfferIDs: declineIds,
			Filters: &mesos.Filters{
				RefuseSeconds: &declineSeconds,
			},
		}
		err := sch.fw.CallMaster(decline)
		if err != nil {
			log.Error("Call DECLINE error:", err)
		}
	}
}

func (sch *Scheduler) onRescind(rescind *scheduler.Event_Rescind) {
	log.Debug("Received Rescind.")
}

func (sch *Scheduler) onUpdate(update *scheduler.Event_Update) {
	if update.Status.Message == nil {
		log.Debugf("Received UPDATE Task: %s, State: %s, UUID: %v",
			update.Status.TaskID.Value,
			update.Status.State.String(),
			update.Status.UUID)
	} else {
		log.Debugf("Received UPDATE Task: %s, State: %s, UUID: %v, Message: %s",
			update.Status.TaskID.Value,
			update.Status.State.String(),
			update.Status.UUID,
			*update.Status.Message)
	}

	status := &cc.TaskStatus{
		Id:         update.Status.TaskID.Value,
		AgentId:    update.Status.AgentID.Value,
		Message:    update.Status.Message,
		StatusTime: time.Now(),
		State:      getTaskMesosStatus(update.Status.State),
	}

	sch.clusterManager.UpdateTaskStatus(status)

	if len(update.Status.UUID) > 0 {
		ack := &scheduler.Call_Acknowledge{
			AgentID: *update.Status.AgentID,
			TaskID:  update.Status.TaskID,
			UUID:    update.Status.UUID,
		}
		err := sch.fw.CallMaster(ack)
		if err != nil {
			log.Error("Call Acknowledge error:", err)
		}
	} else {
		log.Warning("Received UPDATE task event has not UUID, task:",
			update.Status.TaskID.Value)
	}
}

func (sch *Scheduler) onUpdateOperationStatus(update *scheduler.Event_UpdateOperationStatus) {
	// if update.Status.Message == nil {
	// 	log.Debugf("Received UPDATE operation: %s, State: %s, UUID: %v",
	// 		update.Status.OperationID.Value,
	// 		update.Status.State.String(),
	// 		update.Status.UUID)
	// } else {
	// 	log.Debugf("Received UPDATE operation: %s, State: %s, UUID: %v, Message: %s",
	// 		update.Status.OperationID.Value,
	// 		update.Status.State.String(),
	// 		update.Status.UUID,
	// 		*update.Status.Message)
	// }

	data, err := json.Marshal(update)
	if err != nil {
		log.Errorf("Error:%s", err.Error())
	} else {
		log.Errorf("onUpdateOperationStatus: %s", string(data))
	}

	if update.Status.UUID != nil && len(update.Status.UUID.Value) > 0 {
		ack := &scheduler.Call_AcknowledgeOperationStatus{
			AgentID:            update.Status.AgentID,
			ResourceProviderID: update.Status.ResourceProviderID,
			OperationID:        *update.Status.OperationID,
		}
		if update.Status.UUID != nil {
			ack.UUID = update.Status.UUID.Value
		}
		err := sch.fw.CallMaster(ack)
		if err != nil {
			log.Error("Call AcknowledgeOperationStatus error:", err)
		}
	} else {
		log.Warning("Received UPDATE operation event has not UUID, OperationID:",
			update.Status.OperationID.Value)
	}
}

func (sch *Scheduler) onMessage(message *scheduler.Event_Message) {
	log.Debug("Received MESSAGE. ")
}

func (sch *Scheduler) onFailure(failure *scheduler.Event_Failure) {
	//TODO主机失败的场景未实现
	errMsg := "Received FAILURE "
	if failure.AgentID != nil {
		errMsg += "SlaveId:" + failure.AgentID.Value
	}
	if failure.Status != nil {
		errMsg += " Status:" + strconv.Itoa(int(*failure.Status))
	}
	if failure.ExecutorID != nil {
		errMsg += " ExecutorId:" + failure.ExecutorID.Value
	}
	log.Warning(errMsg)
}

func (sch *Scheduler) onError(err *scheduler.Event_Error) {
	log.Debug("Received event: ERROR")
}

func (sch *Scheduler) onHeartbeat() {
	log.Debug("Received event: HEARTBEAT")
}

func (sch *Scheduler) onRegisted(fwID string, mesosVersion *string) {
	log.Info("Framework registed Id: " + fwID)

	sch.saveFwIDFunc(fwID)
	sch.mesosVersion = *mesosVersion

	reconcile := &scheduler.Call_Reconcile{
		Tasks: nil,
	}
	err := sch.fw.CallMaster(reconcile)
	if err != nil {
		log.Error("Call Reconcile error:", err)
	}
}

func (sch *Scheduler) receive(event *scheduler.Event) {
	switch event.Type {
	case scheduler.Event_OFFERS:
		sch.onOffers(event.Offers)
		break
	case scheduler.Event_RESCIND:
		sch.onRescind(event.Rescind)
		break
	case scheduler.Event_UPDATE:
		sch.onUpdate(event.Update)
		break
	case scheduler.Event_UPDATE_OPERATION_STATUS:
		sch.onUpdateOperationStatus(event.UpdateOperationStatus)
		break
	case scheduler.Event_MESSAGE:
		sch.onMessage(event.Message)
		break
	case scheduler.Event_FAILURE:
		sch.onFailure(event.Failure)
		break
	case scheduler.Event_ERROR:
		sch.onError(event.Error)
		break
	case scheduler.Event_HEARTBEAT:
		sch.onHeartbeat()
		break
	case scheduler.Event_SUBSCRIBED:
		sch.onRegisted(event.Subscribed.FrameworkID.Value, event.Subscribed.MasterInfo.Version)
		break
	case scheduler.Event_UNKNOWN:
		log.Warning("Received unknown event")
		break
	default:
		log.Warning("Received unknown event:", event.Type)
		break
	}
}

//Run 执行该scheduler的主循环
func (sch *Scheduler) Run() (err error) {
	if sch == nil {
		return errors.New("Framework is not initialized ")
	}
	return sch.fw.run()
}

//StopTask 停止该framework中的指定任务
func (sch *Scheduler) StopTask(task *cc.Task) (err error) {
	if task == nil || task.Status == nil {
		return
	}
	kill := &scheduler.Call_Kill{
		TaskID: mesos.TaskID{
			Value: task.Status.Id,
		},
		AgentID: &mesos.AgentID{
			Value: task.Status.AgentId,
		},
	}
	return sch.fw.CallMaster(kill)
}

//StopOffer 停止资源分配
func (sch *Scheduler) stopOffer() error {
	sch.enableOfferLock.Lock()
	defer sch.enableOfferLock.Unlock()
	if sch.enableOffer {
		suppress := &scheduler.Call_Suppress{}
		if err := sch.fw.CallMaster(suppress); err != nil {
			return err
		}
		sch.enableOffer = false
		log.Info("Supress offers ok")
	}
	return nil
}

//ReviveOffer 恢复资源分配
func (sch *Scheduler) ReviveOffer() error {
	sch.enableOfferLock.Lock()
	defer sch.enableOfferLock.Unlock()
	if !sch.enableOffer {
		revive := &scheduler.Call_Revive{}
		if err := sch.fw.CallMaster(revive); err != nil {
			log.Errorf("Revive error: %s", err.Error())
			return err
		}
		sch.enableOffer = true
		log.Info("Revive offers ok")
	}
	return nil
}

//GetFrameworkID 得到framework的id
func (sch *Scheduler) GetFrameworkID() string {
	if sch != nil && sch.fw.FrameworkID != nil {
		return sch.fw.FrameworkID.Value
	}
	return ""
}

//GetShortFrameworkID 得到framework的简化id
func (sch *Scheduler) GetShortFrameworkID() string {
	if sch != nil && sch.fw.FrameworkID != nil {
		return utils.HashString(sch.fw.FrameworkID.Value)
	}
	return ""
}

//Close 关闭该scheduler以及对应的http connection
func (sch *Scheduler) Close() {
	if sch != nil {
		sch.fw.close()
	}
}
