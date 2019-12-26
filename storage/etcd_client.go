package storage

import (
	//"go.storage.io/storage/clientv3"
	//"go.storage.io/storage/clientv3/concurrency"
	"cke/log"
	"context"
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/concurrency"
	//"strings"
	"time"

	"strings"
)

type EtcdClient struct {
	etcdCli     *clientv3.Client
	session     *concurrency.Session
	isRunning   bool
	frameworkName *string
	failoverTimeOut *int64
	clusterName *string
	myId        *string
    leader      string
	isLeader    bool
}

func NewEtcdClient(frameworkName *string,failoverTimeOut *int64, etcds []string) (client Storage, err error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   etcds, //[]string{"localhost:2379", "localhost:22379", "localhost:32379"},
		DialTimeout: 3 * time.Second,
	})
	if err != nil {
		return
	}
	client = &EtcdClient{
		etcdCli:     cli,
		isRunning:   true,
		frameworkName: frameworkName,
		failoverTimeOut: failoverTimeOut,
		isLeader: false,
	}
	return
}

func (ec *EtcdClient) Close() {
	ec.isRunning = false
	if ec.session != nil {
		ec.session.Close()
		ec.session = nil
	}
	if ec.etcdCli != nil {
		ec.etcdCli.Close()
		ec.etcdCli = nil
	}
}


func (ec *EtcdClient) getCkePath() string {
	return "/CKE/" + *ec.frameworkName
}

func (ec *EtcdClient) getClusterPath() string {
	return "/CKE/" + *ec.clusterName
}

func (ec *EtcdClient) IsLeader() bool{
	return ec.isLeader
}

func (ec *EtcdClient) GetLeader() string {
	return ec.leader
}

func (ec *EtcdClient) getLeaderKey() string {
	return ec.getCkePath() + "/leader"
}

func (ec *EtcdClient) getFrameworkIdKey() string {
	return ec.getCkePath() + "/framework/ID"
}

func (ec *EtcdClient) getTasksPath() string {
	return ec.getClusterPath() + "/tasks"
}


func (ec *EtcdClient) BecomeLeader(leaderId *string) error {
	//var s *concurrency.Session
	var err error
	//leader 失联后5秒开始进行新的leader选举
	ec.session, err = concurrency.NewSession(ec.etcdCli)
	if err != nil {
		return err
	}

	e := concurrency.NewElection(ec.session, ec.getLeaderKey())
	log.Info("Waiting to be a leader")

	//循环监控leader切换
	go ec.watchLeader(e)

	// become leader
	if err = e.Campaign(context.Background(), *leaderId); err != nil {
		return err
	}

	log.Info("Has become a leader")
	ec.isLeader = true
	// get the leadership details of the current election
	//var leader *clientv3.GetResponse
	// _, err = e.Leader(ctx)
	// if err != nil {
	// 	log.Fatalf("Leader() returned non nil err: %s", err)
	// 	return err
	// }
	return nil
}

func (ec *EtcdClient) watchLeader(e *concurrency.Election) {
	watchChan := ec.etcdCli.Watch(context.Background(), ec.getLeaderKey(), clientv3.WithPrefix(),clientv3.WithCreatedNotify())
	for {
		chanResp := <- watchChan
		for _,event :=range chanResp.Events{
			if event.Type == clientv3.EventTypeDelete  || event.Type == clientv3.EventTypePut{
				//会自动阻塞key的发现
				rsp, err := e.Leader(context.Background())
				if err != nil{
					log.Errorf("fetch leader error :%v",err.Error())
				}else{
					for _, kv :=range rsp.Kvs{log.Infof("Has detected a new leader at : %v", string(kv.Value))
						ec.leader = string(kv.Value)
					}
				}
			}
		}
	}
}

func (ec *EtcdClient) GetFrameworkId() (*string, error) {
	keyName := ec.getFrameworkIdKey()
	resp, err := ec.etcdCli.Get(context.Background(), keyName)

	if err != nil {
		log.Error("get failed, err:", err)
		return nil, err
	}
	for _, ev := range resp.Kvs {
		if string(ev.Key) == keyName {
			id := string(ev.Value)
			return &id, nil
		}
	}
	return nil, nil
}

func (ec *EtcdClient) SetFrameworkId(fwId string) error {

	lease := clientv3.NewLease(ec.etcdCli)
	grantResp, err := lease.Grant(context.Background(), *ec.failoverTimeOut)
	if err != nil{
		log.Error("get failed, err:", err)
		return err
	}
	_, err = ec.etcdCli.Put(context.Background(), ec.getFrameworkIdKey(), fwId, clientv3.WithLease(grantResp.ID))
	if err != nil {
		log.Error("get failed, err:", err)
		return err
	}
	go func() {
		keepChan,err := ec.etcdCli.KeepAlive(context.Background(), grantResp.ID)
		if err != nil{
			log.Errorf("Error from keep alive when set framework id...., error: %v",err.Error())
			return
		}
		for range keepChan{
			// wait to eat chan message....
		}
	}()
	return nil
}

func (ec *EtcdClient) GetClusterAndTypes()(map[string]string, error) {
	clutsPath := ec.getCkePath()+"/clusters"
	rsp,err := ec.etcdCli.Get(context.Background(), clutsPath, clientv3.WithPrefix());

	if err != nil {
		return nil, err
	}

	clusAndTypes := make(map[string]string)
	for _,kv := range rsp.Kvs {
		tmpStr := strings.TrimPrefix(string(kv.Key), clutsPath+"/")
        tmpStr  = tmpStr[:strings.Index(tmpStr,"/")]
		keySplits := strings.Split(tmpStr, ":")
		clusAndTypes[keySplits[1]] = keySplits[0]
	}
	return clusAndTypes,nil
}


func (ec *EtcdClient) GetTasks(call func(taskName string, task []byte)) error {
	keyPrefix := ec.getTasksPath()
	resp, err := ec.etcdCli.Get(context.Background(), keyPrefix, clientv3.WithPrefix())
	if err != nil {
		log.Error("get failed, err:", err)
		return err
	}
	for _, ev := range resp.Kvs {
		taskName := string(ev.Key[len(keyPrefix)+1:])
		log.Info("Get task from etcd:" + taskName)
		call(string(ev.Key), ev.Value)
	}
	return nil
}

func (ec *EtcdClient) AddTask(taskName string, task []byte) error {
	_, err := ec.etcdCli.Put(context.Background(), ec.getTasksPath()+"/"+taskName, string(task))
	if err != nil {
		log.Error("put failed, err:", err)
		return err
	}
	return nil
}

func (ec *EtcdClient) SetTask(taskName string, task []byte) error {
	_, err := ec.etcdCli.Put(context.Background(), ec.getTasksPath()+"/"+taskName, string(task))
	if err != nil {
		log.Error("put failed, err:", err)
		return err
	}
	return nil
}

func (ec *EtcdClient) RemoveTask(taskName string) error {
	_, err := ec.etcdCli.Delete(context.Background(), ec.getTasksPath()+"/"+taskName)
	if err != nil {
		log.Error("delete failed, err:", err)
		return err
	}
	return nil
}
