package container

import (
	netplug "cke/vm/driver/network"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/libnetwork/ipamapi"
	ipam "github.com/docker/libnetwork/ipams/remote/api"
    "net"
    "strconv"
    "strings"
    "testing"
)

/*****************
  Author: Chen
  Date: 2019/4/26
  Comp: ChinaUnicom
 ****************/
func TestGetDockerNetworkInfoByName(t *testing.T) {
	nwName := "spidernet-vpc47403cke"
	networkInfo, err := GetDockerNetworkInfoByName(nwName)

	if err != nil {
		t.Fatalf("GetDockerNetworkInfoByName: %s", err)
	}

	if networkInfo == nil {
		t.Fatalf("Cannot find docker network info about: %s", nwName)
	}
	t.Logf("Ipam infor: %s", networkInfo.IPAM.Options)
}

func TestIpamFullRequestAndRelease(t *testing.T) {
	var netClient *plugins.Client
	netPlugin, err := plugins.Get(netplug.NETPLUGIN, ipamapi.PluginEndpointType)
	if err != nil {
		t.Errorf("Cannot connect to contiv network plugin [%s]", err)
		return
	}

	netClient = netPlugin.Client()

	// 获取ipam池
	method := ipamapi.PluginEndpointType + ".GetDefaultAddressSpaces"
	res := &ipam.GetAddressSpacesResponse{}
	netClient.CallWithOptions(method, nil, res)

	if res.IsSuccess() {
		// request pool
		method = ipamapi.PluginEndpointType + ".RequestPool"
		poolReq := &ipam.RequestPoolRequest{
			Options: map[string]string{
				"network": "spidernet",
				"tenant":  "vpc47403cke",
			},
			Pool: "192.168.0.0/18",
		}
		poolRes := &ipam.RequestPoolResponse{}
		netClient.CallWithOptions(method, poolReq, poolRes)

		if poolRes.IsSuccess() {
			// request addr
			method = ipamapi.PluginEndpointType + ".RequestAddress"
			addReq := &ipam.RequestAddressRequest{
				PoolID: poolRes.PoolID,
			}
			addRes := &ipam.RequestAddressResponse{}
			netClient.CallWithOptions(method, addReq, addRes)

			if addRes.IsSuccess() {
				t.Logf("RequestAddress success: %s", addRes)
			} else {
				t.Logf("RequestAddress Error. %s", res)
			}

            _, ipNet, _:= net.ParseCIDR(addRes.Address)
            val := make([]byte, len(ipNet.Mask))
            copy(val, ipNet.Mask)

            var s []string
            for _, i := range val[:] {
                s = append(s, strconv.Itoa(int(i)))
            }
            t.Logf("INET info : %s, %s", strings.Join(s, "."), ipNet.IP)
			// release addr
			method = ipamapi.PluginEndpointType + ".ReleaseAddress"
			reAddrReq := &ipam.ReleaseAddressRequest{
				PoolID:  poolRes.PoolID,
				Address: addRes.Address,
			}
			reAddrRes := &ipam.ReleaseAddressResponse{}
			netClient.CallWithOptions(method, reAddrReq, reAddrRes)

			if reAddrRes.IsSuccess() {
				t.Logf("ReleaseAddress success: %s", reAddrRes)
			} else {
				t.Logf("ReleaseAddress Error. %s", res)
			}
		} else {
			t.Logf("RequestPool Error. %s", res)
		}
		// release pool
		method = ipamapi.PluginEndpointType + ".ReleasePool"
		rePoolReq := &ipam.ReleasePoolRequest{
			PoolID: poolRes.PoolID,
		}
		rePoolRes := &ipam.ReleaseAddressResponse{}
		netClient.CallWithOptions(method, rePoolReq, rePoolRes)

		if rePoolRes.IsSuccess() {
			t.Logf("ReleasePool success: %s", rePoolRes)
		} else {
			t.Logf("ReleasePool Error. %s", res)
		}
	} else {
		t.Logf("GetDefaultAddress Error. %s", res)
	}
}
