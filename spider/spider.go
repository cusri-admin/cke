package spider

//对接spider网络的胖容器扩缩容rest接口。
import (
	"bytes"
	"cke/log"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

var (
	//LBEndpoint 指定spider的服务端
	LBEndpoint = flag.String("k8s_spider_lb_endpoint", "", "The IP and Port of the spider LB service")
)

//vipResult 查询VIP的数据结果
type vipResult struct {
	Status  int      `json:"status"`  //后端地址列表
	Message string   `json:"message"` //前端地址
	Value   []string `json:"value"`   //Vip address list
}

//LBService LB接口调用参数
type lbService struct {
	VPC       string `json:"vpc"`
	ServiceID string `json:"serviceid"`
	Protocol  string `json:"protocol,omitempty"`
	Scheduler string `json:"scheduler,omitempty"`
	Frontend  string `json:"frontend,omitempty"`
	Backend   string `json:"backend,omitempty"`
}

//postRequest 向VPC添加若干节点IP的请求体
type postRequest struct {
	VPC  string   `json:"vpc"`
	Host []string `json:"host"`
}

func post(vpc string, host []string, url string) error {
	request := &postRequest{
		VPC:  vpc,
		Host: host,
	}
	data, _ := json.Marshal(request)
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("add or delete post http.NewRequest ERROR! err: %s", err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{
		//Timeout: 10000,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("add or delete post client.Do ERROR! err: %s", err.Error())
	}
	defer resp.Body.Close()

	log.Infof("Send post data to spider: %s", string(data))
	if resp.StatusCode > 299 || resp.StatusCode < 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("add or delete post StatusCode: %d, error: %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("add or delete post ioutil.ReadAll ERROR! err: %s", err.Error())
	}

	log.Infof("Add or delete response Body: %s", string(body))
	return nil
}

func deleteLBServicesFromSpider(url string) error {
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	log.Errorf("URL %s status: %d", url, resp.StatusCode)
	if resp.StatusCode >= 299 || resp.StatusCode < 200 {
		return fmt.Errorf("can not delete LB services from spider, http status: %d ", resp.StatusCode)
	}
	if resp != nil {
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		log.Infof("Delete LB services response http status: %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}

//AddHostsFromRR 向spider RR添加contiv ip地址
func AddHostsFromRR(networkName string, host []string) error {
	vpc := getVPCFromNetworkName(networkName)
	if *LBEndpoint != "" && vpc != "" && host != nil && len(host) > 0 {
		log.Infof("Add vpc %s, ip: %v", vpc, host)
		url := *LBEndpoint + "/v1/spider/cke/host/add"
		err := post(vpc, host, url)
		if err != nil {
			return err
		}
		return nil
	}
	return nil
}

//DeleteHostsFromRR 删除spider RR中的contiv IP地址.
func DeleteHostsFromRR(networkName string, host []string) error {
	vpc := getVPCFromNetworkName(networkName)
	if *LBEndpoint != "" && vpc != "" && host != nil && len(host) > 0 {
		log.Infof("Delete vpc %s, ip: %v", vpc, host)
		url := *LBEndpoint + "/v1/spider/cke/host/update"
		err := post(vpc, host, url)
		if err != nil {
			return err
		}
		return nil
	}
	return nil
}

//DeleteLBServicesFromSpider delete lb service form spider
func DeleteLBServicesFromSpider(networkName, serviceID string) error {
	vpc := getVPCFromNetworkName(networkName)
	if *LBEndpoint != "" && vpc != "" {
		spiderLBSvcs, err := getLBServiceFromSpider(networkName, serviceID)
		if err != nil {
			return err
		}
		for _, svc := range spiderLBSvcs {
			url := fmt.Sprintf("%s/v1/spider/lb/service/del/?vpc=%s&serviceid=%s", *LBEndpoint, svc.VPC, svc.ServiceID)
			if err := deleteLBServicesFromSpider(url); err != nil {
				return err
			}
		}
	}
	return nil
}

//获取该集群在lb上所有的注册服务
func getLBServiceFromSpider(networkName, serviceID string) ([]lbService, error) {
	vpc := getVPCFromNetworkName(networkName)
	spiderLBSvRLcURL := *LBEndpoint + "/v1/spider/lb/service/get?vpc=" + vpc
	log.Infof("Get services from spider url: %s", spiderLBSvRLcURL)
	resp, err := http.Get(spiderLBSvRLcURL)
	if err != nil {
		return nil, fmt.Errorf("can not get service from spider: %s, err: %s", spiderLBSvRLcURL, err.Error())
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("can not get service from spider: %s, err: %s", spiderLBSvRLcURL, err.Error())
	}
	defer resp.Body.Close()
	body, err1 := ioutil.ReadAll(resp.Body)
	if err1 != nil {
		return nil, fmt.Errorf("can not read data from  resp.Body, err: %s", err.Error())
	}
	var svcOfVpc []lbService
	err = json.Unmarshal([]byte(body), &svcOfVpc)
	if err != nil {
		return nil, errors.New("json to struct failed")
	}
	var lbServices []lbService
	for _, svc := range svcOfVpc {
		if strings.HasPrefix(svc.ServiceID, serviceID) {
			lbServices = append(lbServices, svc)
		}
	}
	return lbServices, nil
}

//GetVIP 获得指定网络的VIP
func GetVIP(networkName string) (string, error) {
	vpc := getVPCFromNetworkName(networkName)
	spiderLBUrl := *LBEndpoint + "/v1/spider/lb/vip/get?vpcTenant=" + vpc
	log.Infof("Get VIP from spider url: %s", spiderLBUrl)
	resp, err := http.Get(spiderLBUrl)
	if err != nil {
		return "", fmt.Errorf("can not get data from spider url: %s, error: %s", spiderLBUrl, err.Error())
	}
	defer resp.Body.Close()
	body, err1 := ioutil.ReadAll(resp.Body)
	if err1 != nil {
		return "", fmt.Errorf("can not read data from resp.Body error: %s", err.Error())
	}
	var result vipResult
	err = json.Unmarshal([]byte(body), &result)
	if err != nil {
		return "", fmt.Errorf("json to struct failed error: %s", err.Error())
	}
	for _, ip := range result.Value {
		if ip != "" {
			log.Infof("spider vip: %s", ip)
			return ip, nil
		}
	}
	return "", fmt.Errorf("can not find vip in network: %s", networkName)
}

func getVPCFromNetworkName(networkName string) string {
	var vpc string = ""
	kv := strings.Split(networkName, "-")
	if len(kv) >= 2 {
		vpc = kv[len(kv)-1]
	}
	return vpc
}
