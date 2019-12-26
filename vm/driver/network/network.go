package network

import (
	"cke/log"
	"cke/vm/model"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/reexec"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

/*****************
  Author: Chen
  Date: 2019/4/26
  Comp: ChinaUnicom
 ****************/

const (
	// DEFAULT : default bridge network
	DEFAULT = "default"
	// NETPLUGIN : docker contiv plugin name
	NETPLUGIN = "netplugin"
	// CONTIV : name of contiv plugin
	CONTIV = "contiv"
	// OVS : ovs plugin
	OVS = "ovs"

	defaultPrefix = "/var/run/cke"
)

var (
	once   sync.Once
	prefix = defaultPrefix
)

type Network interface {
	GetName() string
	//创建删除Endpoint
	CreateEndpoint(networkInfo *types.NetworkResource, endpointID string) (*Endpoint, error)
	DeleteEndpoint(networkInfo *types.NetworkResource, endpoint *Endpoint) error
	// CreateNetwork 创建删除Network
	CreateNetwork(netConf *model.NetInfo) (*types.NetworkResource, error)
	DeleteNetwork() error
	//创建删除Tap设备
	CreateVMTap() error
	DeleteVMTap() error
	//创建删除bridge
	CreateBridge(brName string) error
	DeleteBridge(brName string) error
	//绑定与解绑 桥与interface
	BindInterfaceToBridge(brName string, netId string, epId string) (map[string]interface{}, error)
	UnBindInterfaceFromBridge(brName string, netId string, endpointID string, ifName string) error
}

type Endpoint struct {
	Id       string
	BrName   string
	EpName   string
	TapName  string
	Ipv4Addr string
	Ipv4Net  string // 网段
	Ipv4Mask string // 掩码
	Ipv6Addr string
	Ipv6Mask string
	MacAddr  string
	HostName string
	Gateway  string
}

type NetworkContext struct {
	Endpoints map[string]*Endpoint
	Drivers   map[string]Network
}

func init() {
	/*reexec.Register("netns-create", reexecCreateNamespace)
	  if reexec.Init() {
	      os.Exit(0)
	  }*/
}

func (netContext *NetworkContext) RegisterNetwork(name string, network interface{}) {
	netContext.Drivers[name] = network.(Network)
}

func GenerateRandomId() string {
	unix32bits := uint32(time.Now().UTC().Unix())
	buff := make([]byte, 8)
	numRead, err := rand.Read(buff)

	if numRead != len(buff) || err != nil {
		return ""
	}
	return fmt.Sprintf("%x", unix32bits)
}

func GenRandomMac() (string, error) {
	const alphanum = "0123456789abcdef"
	var bytes = make([]byte, 8)
	_, err := rand.Read(bytes)

	if err != nil {
		log.Error("get random number faild")
		return "", err
	}

	for i, b := range bytes {
		bytes[i] = alphanum[b%byte(len(alphanum))]
	}

	tmp := []string{"52:54", string(bytes[0:2]), string(bytes[2:4]), string(bytes[4:6]), string(bytes[6:8])}
	return strings.Join(tmp, ":"), nil
}

func GenerateNameSpacePath(nsName string) string {
	return basePath() + "/" + nsName
}

func CreateNetworkNamespace(path string, osCreate bool) error {
	if err := createNamespaceFile(path); err != nil {
		return err
	}

	cmd := &exec.Cmd{
		Path:   reexec.Self(),
		Args:   append([]string{"netns-create"}, path),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	if osCreate {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
		cmd.SysProcAttr.Cloneflags = syscall.CLONE_NEWNET
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("namespace creation reexec command failed: %v", err)
	}

	return nil
}

func RemoveNetworkNamespace(path string) {
	// unmount ns
	// 删除ns
	unmountNamespaceFile(path)
	removeNameSpacePath(path)
}

func mountNetworkNamespace(basePath string, lnPath string) error {
	return syscall.Mount(basePath, lnPath, "bind", syscall.MS_BIND, "")
}

func unmountNamespaceFile(path string) {
	if _, err := os.Stat(path); err == nil {
		syscall.Unmount(path, syscall.MNT_DETACH)
	}
}

func createNamespaceFile(path string) (err error) {
	var f *os.File

	once.Do(createBasePath)
	// Remove it from garbage collection list if present
	//removeFromGarbagePaths(path)

	// If the path is there unmount it first
	// unmountNamespaceFile(path) TODO: revert

	// wait for garbage collection to complete if it is in progress
	// before trying to create the file.
	//gpmWg.Wait()

	if f, err = os.Create(path); err == nil {
		f.Close()
	}

	return err
}

func reexecCreateNamespace() {
	if len(os.Args) < 2 {
		log.Error("no namespace path provided")
	}
	if err := mountNetworkNamespace("/proc/self/ns/net", os.Args[1]); err != nil {
		log.Error(err)
	}
}

func basePath() string {
	return filepath.Join(prefix, "netns")
}

func SetBasePath(path string) {
	prefix = path
}

func createBasePath() {
	err := os.MkdirAll(basePath(), 0755)
	if err != nil {
		panic("Could not create net namespace path directory")
	}

	// Start the garbage collection go routine
	//go removeUnusedPaths()
}

func removeNameSpacePath(path string) {
	os.Remove(path)
	/*gpmLock.Lock()
	  period := gpmCleanupPeriod
	  gpmLock.Unlock()

	  ticker := time.NewTicker(period)
	  for {
	      var (
	          gc   chan struct{}
	          gcOk bool
	      )

	      select {
	      case <-ticker.C:
	      case gc, gcOk = <-gpmChan:
	      }

	      gpmLock.Lock()
	      pathList := make([]string, 0, len(garbagePathMap))
	      for path := range garbagePathMap {
	          pathList = append(pathList, path)
	      }
	      garbagePathMap = make(map[string]bool)
	      gpmWg.Add(1)
	      gpmLock.Unlock()

	      for _, path := range pathList {
	          os.Remove(path)
	      }

	      gpmWg.Done()
	      if gcOk {
	          close(gc)
	      }
	  } */
}
