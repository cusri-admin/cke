package driver

import (
	"cke/vm/driver/network"
	"cke/vm/driver/storage"
	"cke/vm/image"
	"cke/vm/model"
)

// 创建系统磁盘
func CreateSystemDisk(vm *model.LibVm, sc *storage.StorageContext) (*map[string]string, error) {
	if vm.BootType == model.BootType_BOOT_SHARE {
		// 创建secret, 创建 cbs volume, 返回
		keyRing := ""
		secret, err := sc.R.CreateCephSecret(vm.Label["user"], keyRing)
		if err != nil {
			return nil, err
		}

		v, err := sc.R.CreateSystemVolume(vm)
		if err != nil {
			// TODO 回滚删除 secret
			secret.Undefine()
			return nil, err
		}
		uuid, _ := secret.GetUUIDString()
		(*v)["uuid"] = uuid
		(*v)["devName"] = "vda"
		return v, nil
	} else { //使用本地文件作为启动盘
		//  local
		res, err := sc.L.CreateSystemVolume(vm)
		if err != nil {
			return nil, err
		}
		(*res)["devName"] = "vda"
		return res, nil
	}
}

// 创建元数据盘
func CreateMetaDisk(vm *model.LibVm, endpoint *network.Endpoint, sc *storage.StorageContext) (*map[string]string, error) {
	// 从 用户配置中获取 password, user-data, hostname
	// 从 network中 获取 network info
	userdata := vm.OsInfo["userData"]
	password := vm.OsInfo["password"]
	hostname := vm.OsInfo["hostname"]
	metadata := &image.Metadata{
		InstanceId:    vm.Name,
		IfName:        "eth0",
		Ipv4Addr:      endpoint.Ipv4Addr,
		Ipv4Network:   endpoint.Ipv4Net,
		Ipv4Mask:      endpoint.Ipv4Mask,
		Gateway:       endpoint.Gateway,
		Hostname:      hostname,
		Password:      password,
		IsPwdExpire:   false,
	}

	var metaMap *map[string]string

	if vm.BootType == model.BootType_BOOT_SHARE {
		metaPath, err := sc.R.CreateMetaVolume(metadata, &userdata)
        if err != nil || metaPath == ""{
            return nil, err
        }
        metaMap = &map[string]string{
            "devName": "vdb",
            "path": metaPath,
        }
	} else {
        metaPath, err := sc.L.CreateMetaVolume(metadata, &userdata)
        if err != nil || metaPath == "" {
            return nil, err
        }
        metaMap = &map[string]string{
            "devName": "vdb",
            "path": metaPath,
        }
	}
	return metaMap, nil
}

// 删除磁盘
func DeleteDisk(vm *model.LibVm, sc *storage.StorageContext) (*map[string]string, error) {
	// 删除cbs volume, 删除secret, 返回
	if vm.BootType == model.BootType_BOOT_SHARE {
		if err := sc.R.DeleteCBSVolume(vm.Label["user"], vm.Region, vm.Id); err != nil {
			return nil, err
		}
		// TODO 删除 secret
		return nil, nil
	} else { //使用本地文件作为启动盘
		//  local
		res, err := sc.L.DeleteVolume(vm)
		if err != nil {
			return nil, err
		}
		return res, nil
	}
}

func CreateDataDisk(vm *model.LibVm, sc *storage.StorageContext) (*map[string]string, error) {
	return nil, nil
}
