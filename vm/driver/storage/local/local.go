package local

import (
    "cke/log"
    "cke/vm/image"
    "cke/vm/model"
    "os"
)

/*****************
  Author: Chen
  Date: 2019/8/9
  Comp: ChinaUnicom
 ****************/

type DiskDriver struct {
    Name string
    Type string
}

// 拉取镜像 --> 创建volume -> 制作系统盘 -> 成功
func (d *DiskDriver) CreateSystemVolume(vm *model.LibVm) (*map[string]string, error) {
    disk := map[string]string{}
    diskImgFormat := "raw"
    diskPath := "/data/" + vm.Name + "." + diskImgFormat //生成的卷路径，保证路径可写
    resStatus, err := image.CreateBootImg(vm.OsInfo["imgPath"], diskPath, diskImgFormat)
    if err != nil || !resStatus {
        log.Errorf("copy image to volume error: %s", err)
        return nil, err
    }

    disk["format"] = diskImgFormat
    disk["path"] = diskPath

    if vm.Disk.SystemDisk.Info == nil {
        vm.Disk.SystemDisk.Info = map[string]string{}
    }
    vm.Disk.SystemDisk.Info["path"] = diskPath
    vm.Disk.SystemDisk.Info["diskImgFormat"] = diskImgFormat
    vm.Disk.SystemDisk.Name = vm.Name
    vm.Disk.SystemDisk.Size = 0
    return &disk, nil
}

func (d *DiskDriver) CreateMetaVolume (mdata *image.Metadata, udata *string) (string, error){
    metaData, err := image.GetMetaTemplate()
    if err != nil {
        log.Errorf("Meta file parse error : %s", err)
        return "", err
    }
    dir := "/tmp/cke-kvm/"+ mdata.InstanceId
    os.MkdirAll(dir, 777)
    metafile := dir + "/meta-data"
    if f, err := os.Create(metafile); err == nil {
        if err = metaData.Execute(f, mdata); err != nil {
            return "", err
        }
    }

    userfile := dir + "/user-data"
    if f, err := os.Create(userfile); err == nil {
        if *udata != "" {
            if _, err = f.WriteString(*udata); err != nil {
                return "", err
            }
        }
    }
    // 创建 NoCloud ISO
    destPath := "/data/" + mdata.InstanceId + "-seed.iso"
    err = image.CreateMetaImage(metafile, userfile, destPath)
    if err != nil {
        return "", err
    }
    // 删除临时目录， 并返回结果路径
    return destPath, os.RemoveAll(dir)
}

func (d *DiskDriver) DeleteVolume(vm *model.LibVm) (*map[string]string, error){
    err := deleteSystemDisk(vm)
    err = deleteMetaDisk(vm)
    // TODO： 删除系统磁盘
    return nil, err
}

// 删除元数据盘
func deleteMetaDisk(vm *model.LibVm) (error) {
    diskPath := "/data/" + vm.Name + "-seed.iso"
    err := os.Remove(diskPath)
    if err != nil {
        return err
    }
    return nil
}

// 删除系统磁盘
func deleteSystemDisk(vm *model.LibVm) (error) {
    diskPath := "/data/" + vm.Name + "." + vm.Disk.SystemDisk.Info["diskImgFormat"]
    err := os.Remove(diskPath)
    if err != nil {
        return err
    }
    return nil
}
