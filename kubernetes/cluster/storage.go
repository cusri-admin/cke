package cluster

import (
	"cke/cluster"
	"cke/storage"
)

type Storage interface {
	Close() error
	StoreCluster(cluster *Cluster) error
	FetchCluster(cluster *Cluster) error
	DeleteCluster(cluster *Cluster) error
	StoreSharedata(shareData *ShareData) error
	FetchSharedata(shareData *ShareData, nodes []*KubeNode) error
	StoreKubeNode(node *KubeNode) error
	FetchKubenode(node *KubeNode) error
	DeleteKubenode(node *KubeNode) error
	StoreProcess(node *KubeNode, process *Process) error
	FetchProcess(node *KubeNode, process *Process) error
	StoreTask(node *KubeNode, process *Process, task *cluster.Task) error
	FetchTask(node *KubeNode, process *Process, task *cluster.Task) error
	StoreReservation(node *KubeNode, reservation *cluster.Reservation) error
	FetchReservation(node *KubeNode) error
	StoreConf(cluster *Cluster) error
	FetchConf(cluster *Cluster) error
}

func CreateStorage(clusterName string, storageType storage.StorageType) Storage {
	switch storageType {
	case storage.StorageType_Etcd:
		return NewEtcdClient(&clusterName)
	case storage.StorageType_Zookeeper:
		return nil
	case storage.StorageType_Empty:
		return &EmptyStorage{}
	}
	return &EmptyStorage{}
}

type EmptyStorage struct {
}

func (es *EmptyStorage) Close() error                                                 { return nil }
func (es *EmptyStorage) StoreCluster(cluster *Cluster) error                          { return nil }
func (es *EmptyStorage) FetchCluster(cluster *Cluster) error                           { return nil }
func (es *EmptyStorage) DeleteCluster(cluster *Cluster) error                          { return nil }
func (es *EmptyStorage) StoreSharedata(shareData *ShareData) error                    { return nil }
func (es *EmptyStorage) FetchSharedata(shareData *ShareData, nodes []*KubeNode) error { return nil }
func (es *EmptyStorage) StoreKubeNode(node *KubeNode) error                           { return nil }
func (es *EmptyStorage) FetchKubenode(node *KubeNode) error                           { return nil }
func (es *EmptyStorage) DeleteKubenode(node *KubeNode) error                          { return nil }
func (es *EmptyStorage) StoreProcess(node *KubeNode, process *Process) error          { return nil }
func (es *EmptyStorage) FetchProcess(node *KubeNode, process *Process) error          { return nil }
func (es *EmptyStorage) StoreTask(node *KubeNode, process *Process, task *cluster.Task) error {
	return nil
}
func (es *EmptyStorage) FetchTask(node *KubeNode, process *Process, task *cluster.Task) error {
	return nil
}
func (es *EmptyStorage) StoreReservation(node *KubeNode, reservation *cluster.Reservation) error {return  nil}
func (es *EmptyStorage) FetchReservation(node *KubeNode) error {return  nil}

func (es *EmptyStorage) StoreConf(cluster *Cluster) error {return nil}
func (es *EmptyStorage) FetchConf(cluster *Cluster) error {return nil}