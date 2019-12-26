package storage

import (
    "cke/vm/driver/storage/local"
    "cke/vm/driver/storage/remote"
    "cke/vm/model"
)

type Storage interface {
    CreateVolume(vm *model.LibVm) (*map[string]string, error) // 创建卷并绑定到宿主机
    DeleteVolume(vm *model.LibVm) (*map[string]string, error) // 从宿主机移除卷并删除
}

type StorageContext struct {
    L *local.DiskDriver
    R *remote.DiskDriver
}
