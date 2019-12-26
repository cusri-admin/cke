package rsa

import (
	"time"
	"crypto/md5"
	"io"
	"strconv"
	"fmt"
)

func CreateToken() string{
	cruTime := time.Now().UnixNano()
	h := md5.New()
	io.WriteString(h, strconv.FormatInt(cruTime, 10))
	token := fmt.Sprintf("%x", h.Sum(nil))
	return token
}
