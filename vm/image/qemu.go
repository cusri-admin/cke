package image

import (
    "cke/log"
    "github.com/pkg/errors"
    "os/exec"
    "strings"
)

type ImageInfo struct {
    VirSize    int64  `json:"virtual-size"`
    FileName   string `json:"filename"`
    Format     string `json:"format"`
    ActualSize int64  `json:"actual-size"`
    DirtyFlag  bool   `json:"dirty-flag"`
}

func CreateBootImg(srcPath string, desPath string, outPutFormat string) (bool, error) {
    //TODO: 1. 下载Image

    //2. 复制Image到指定的Volume
    var isNone string
    var cmd *exec.Cmd
    if strings.HasPrefix(desPath, "rbd:") {
        isNone = "-n"
    }
    cmdStr, err := exec.LookPath("qemu-img")
    if err != nil {
        return false, errors.New("cannot find qemu-img, install lib first")
    }
    if isNone == "" { //file
        cmd = exec.Command(cmdStr, "convert", "-O", outPutFormat, "-f", "qcow2", srcPath, desPath)
    } else { //rbd：-n skips the target volume creation
        cmd = exec.Command(cmdStr, "convert", "-O", outPutFormat, isNone, "-f", "qcow2", srcPath, desPath)
    }
    err = cmd.Run()
    if err != nil {
        log.Errorf("qemu-img convert error occured.")
        return false, err
    }
    return true, nil
}
