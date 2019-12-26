package storage

type StorageType string;

var(
   StorageType_Etcd  StorageType= "etcd"
   StorageType_Zookeeper  StorageType= "zookeeper"
   StorageType_Empty  StorageType= "empty"
)


type Storage interface {
   Close()
   getCkePath() string
   IsLeader() bool
   GetLeader() string
   getLeaderKey() string
   getFrameworkIdKey() string
   BecomeLeader(*string) error
   GetFrameworkId() (*string, error)
   SetFrameworkId(string) error
   GetClusterAndTypes()(map[string]string, error)
}

func CreateStorage(storageType StorageType, frameworkName *string,failoverTimeOut *int64, etcds []string) (Storage, error){
   switch storageType{
   case StorageType_Etcd:
      if client,err := NewEtcdClient(frameworkName,failoverTimeOut, etcds); err == nil{
         return client, nil
      }else{
         return nil,err
      }
      break;
   case StorageType_Zookeeper:
      return nil,nil;
      break;
   case StorageType_Empty:
      return &EmptyStorage{},nil
      break

   }
   return &EmptyStorage{},nil
}

type EmptyStorage struct {}

func (es *EmptyStorage) Close() { return }
func (es *EmptyStorage) getCkePath() string{return ""}
func (es *EmptyStorage) IsLeader() bool{return true}
func (es *EmptyStorage) GetLeader() string{return ""}
func (es *EmptyStorage) getLeaderKey() string{return ""}
func (es *EmptyStorage) getFrameworkIdKey() string{return ""}
func (es *EmptyStorage) BecomeLeader(*string) error{return nil}
func (es *EmptyStorage) GetFrameworkId() (*string, error){return nil,nil}
func (es *EmptyStorage) SetFrameworkId(string) error{return nil}
func (es *EmptyStorage) GetClusterAndTypes()(map[string]string, error){return nil,nil}

