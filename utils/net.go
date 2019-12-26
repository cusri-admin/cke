package utils

import (
	"net"
	"os"
)

func GetHostIp() (string, error) {
	host, err := os.Hostname()
	if err != nil {
		return "", err
	}
	conn, err := net.Dial("ip:icmp", host)
	if err != nil {
		return "", err
	}
	add := conn.RemoteAddr()
	return add.String(), nil
}
