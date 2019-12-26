package cluster

import (
	"errors"
	"strconv"
	"strings"
)

//KubeNodeType 标识节点的类型
type IngressType string

//各种ingress类型
const (
	IngressType_Bridge   IngressType = "bridge"
	IngressType_SpiderLB IngressType = "spider-lb"
)

func (t IngressType) String() string {
	return string(t)
}

//IngressPort 定义ingress对应的端口和协议,形如(1234/tcp)
type IngressPort string

//GetInfo 得到IngressPort的端口和协议
func (p *IngressPort) GetInfo() (int32, string, error) {
	if len(string(*p)) == 0 {
		return 0, "", errors.New("null port")
	}
	parts := strings.Split(string(*p), "/")
	l := len(parts)
	if l == 0 || len(parts[0]) == 0 {
		return 0, "", errors.New("null port")
	}
	if l > 2 {
		return 0, "", errors.New("illegal port")
	}

	proto := "tcp"
	if l == 2 {
		if parts[1] != "tcp" && parts[1] != "http" {
			return 0, "", errors.New("illegal proto")
		}
		proto = parts[1]
	}
	d, err := strconv.ParseInt(parts[0], 0, 32)
	if err != nil {
		return 0, "", err
	}
	return int32(d), proto, nil
}

//CheckFormat 检查IngressPort的合法性
func (p *IngressPort) CheckFormat() error {
	if _, _, err := p.GetInfo(); err != nil {
		return err
	}
	return nil
}

//Ingress 集群中创建时自动启动的Ingress的定义
type Ingress struct {
	Type    IngressType   `json:"type"`
	Bridges []IngressPort `json:"bridges"`

	Vpc              string `json:"-"`
	SpiderLBEndpoint string `json:"-"`
	ServiceidPrefix  string `json:"-"`
}

func (igrs *Ingress) String() string {
	var ret string = ""
	if igrs == nil {
		return ret
	}

	isBridge := (strings.Compare(igrs.Type.String(), IngressType_Bridge.String()) == 0)

	if isBridge != true {
		return IngressType_SpiderLB.String() + "/" + "/" + igrs.Vpc + "/" + igrs.SpiderLBEndpoint + "/" + igrs.ServiceidPrefix
	}

	for n, port := range igrs.Bridges {
		if isBridge {
			iport, _, _ := port.GetInfo()
			ingressStr := IngressType_Bridge.String() + "/" + strconv.FormatInt(int64(iport), 10) + "/" + igrs.Vpc + "/" + igrs.SpiderLBEndpoint + "/" + igrs.ServiceidPrefix
			if n > 0 {
				ret += "," + ingressStr
			} else {
				ret = ingressStr
			}
		}
	}

	return ret
}

//GetPortsByNames 获得names指定的ingress的port信息
func (igrs *Ingress) GetIngressPorts() []IngressPort {
	if igrs == nil {
		return nil
	}

	if strings.Compare(igrs.Type.String(), IngressType_Bridge.String()) == 0 {
		return igrs.Bridges
	}

	return nil
}

//CheckFormat 检查Ingress的合法性
func (igrs *Ingress) CheckFormat() error {
	if igrs == nil {
		return nil
	}

	if len(igrs.Type) <= 0 {
		return errors.New("Ingress type is null")
	}

	if strings.Compare(igrs.Type.String(), IngressType_Bridge.String()) != 0 && strings.Compare(igrs.Type.String(), IngressType_SpiderLB.String()) != 0 {
		return errors.New("Ingress type \"" + igrs.Type.String() + "\"" + " is null")
	}

	if strings.Compare(igrs.Type.String(), IngressType_SpiderLB.String()) == 0 {
		return nil
	}

	for _, port := range igrs.Bridges {
		if len(port) <= 0 {
			return errors.New("Ingress port is null")
		}

		if err := port.CheckFormat(); err != nil {
			return errors.New("Ingress port \"" + string(port) + "\" is illegal: " + err.Error())
		}
	}

	if len(igrs.Vpc) == 0 {
		return errors.New("Ingress vpc is null")
	}

	if len(igrs.Vpc) > 1024 {
		return errors.New("Ingress vpc greater than 1024 characters")
	}

	if len(igrs.SpiderLBEndpoint) > 1024 {
		return errors.New("Ingress spiderLBEndpoint greater than 1024 characters")
	}

	if len(igrs.ServiceidPrefix) == 0 {
		return errors.New("Ingress serviceidprefix is null")
	}

	if len(igrs.ServiceidPrefix) > 1024 {
		return errors.New("Ingress serviceidprefix greater than 1024 characters")
	}

	return nil
}
