package storage

import (
	//"go.etcd.io/etcd/clientv3"
	//"go.etcd.io/etcd/clientv3/concurrency"
	"cke/log"
	"context"
	"encoding/json"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/concurrency"

	//"strings"
	"cke/cluster"
	"strings"
	"time"
)

type EtcdClient struct {
	etcdCli     *clientv3.Client
	session     *concurrency.Session
	isRunning   bool
	clusterName *string
	myId        *string
}

func NewClient(clusterName *string) (*EtcdClient, error) {
	if len(cluster.StorageServers) == 0 {
		return &EtcdClient{
			isRunning:   false,
			clusterName: clusterName,
		}, nil

	}
	etcds := strings.Split(cluster.StorageServers, ",")
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   etcds, //[]string{"localhost:2379", "localhost:22379", "localhost:32379"},
		DialTimeout: 3 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	return &EtcdClient{
		etcdCli:     cli,
		isRunning:   true,
		clusterName: clusterName,
	}, err
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

func (ec *EtcdClient) getClusterPath() string {
	return "/CKE/" + *ec.clusterName
}

func (ec *EtcdClient) getLeaderKey() string {
	return ec.getClusterPath() + "/leader"
}

func (ec *EtcdClient) getFrameworkIdKey() string {
	return ec.getClusterPath() + "/frameowrk/ID"
}

func (ec *EtcdClient) getTasksPath() string {
	return ec.getClusterPath() + "/tasks"
}

func (ec *EtcdClient) BecomeLeader(leaderId *string) error {
	//var s *concurrency.Session
	var err error
	ec.session, err = concurrency.NewSession(ec.etcdCli)
	if err != nil {
		return err
	}

	e := concurrency.NewElection(ec.session, ec.getLeaderKey())

	log.Info("Waiting to be a leader")
	// become leader
	if err = e.Campaign(context.Background(), *leaderId); err != nil {
		return err
	}

	log.Info("Has become a leader")

	// get the leadership details of the current election
	//var leader *clientv3.GetResponse
	// _, err = e.Leader(ctx)
	// if err != nil {
	//     log.Fatalf("Leader() returned non nil err: %s", err)
	//     return err
	// }
	return nil
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
	_, err := ec.etcdCli.Put(context.Background(), ec.getFrameworkIdKey(), fwId)
	if err != nil {
		log.Error("get failed, err:", err)
		return err
	}
	return nil
}

func (ec *EtcdClient) GetTasks(call func(taskName string, task *cluster.TaskInfo)) error {
	keyPrefix := ec.getTasksPath()
	resp, err := ec.etcdCli.Get(context.Background(), keyPrefix, clientv3.WithPrefix())
	if err != nil {
		log.Error("get failed, err:", err)
		return err
	}
	for _, ev := range resp.Kvs {
		taskName := string(ev.Key[len(keyPrefix)+1:])
		log.Info("Get task from etcd:" + taskName)
		task := &cluster.TaskInfo{}
		err := json.Unmarshal(ev.Value, task)
		if err != nil {
			return err
		}
		call(string(ev.Key), task)
	}
	return nil
}

func (ec *EtcdClient) SetTask(taskName string, task *cluster.TaskInfo) error {
	vmBytes, err := json.Marshal(task)
	if err != nil {
		return err
	}
	_, err = ec.etcdCli.Put(context.Background(), ec.getTasksPath()+"/"+taskName, string(vmBytes))
	if err != nil {
		log.Error("put failed, err:", err)
		return err
	}
	return nil
}

// func (ec *EtcdClient) SetTask(taskName string, task []byte) error {
// 	_, err := ec.etcdCli.Put(context.Background(), ec.getTasksPath()+"/"+taskName, string(task))
// 	if err != nil {
// 		log.Error("put failed, err:", err)
// 		return err
// 	}
// 	return nil
// }

func (ec *EtcdClient) RemoveTask(taskName string) error {
	_, err := ec.etcdCli.Delete(context.Background(), ec.getTasksPath()+"/"+taskName)
	if err != nil {
		log.Error("delete failed, err:", err)
		return err
	}
	return nil
}
