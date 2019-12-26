package test

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

func timeoutDialer(cTimeout time.Duration, rwTimeout time.Duration) func(net, addr string) (c net.Conn, err error) {
	return func(netw, addr string) (net.Conn, error) {
		conn, err := net.DialTimeout(netw, addr, cTimeout)
		if err != nil {
			return nil, err
		}
		conn.SetDeadline(time.Now().Add(rwTimeout))
		return conn, nil
	}
}

//PostJSONFile2CKE 向CKE post指定的json文件
func PostJSONFile2CKE(urls []string, jsonFile string) error {
	//从文件的json中创建集群
	json, err := ioutil.ReadFile(jsonFile)
	if err != nil {
		return err
	}
	return PostCKE(urls, json)
}

//PostCKE 向CKE post数据
func PostCKE(urls []string, body []byte) error {
	var err error
	for _, url := range urls {
		err = _postCKE(url, body)
		if err == nil {
			return nil
		}
	}
	return err
}
func _postCKE(url string, body []byte) error {
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))

	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	connectTimeout := time.Duration(2*1000) * time.Millisecond
	readWriteTimeout := time.Duration(10*1000) * time.Millisecond
	httpClient := &http.Client{
		//禁止重定向
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			Dial:            timeoutDialer(connectTimeout, readWriteTimeout),
		},
	}
	resp, err := httpClient.Do(req)
	defer func() {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}()
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		buf, err := ioutil.ReadAll(io.LimitReader(resp.Body, 32*1024))
		if err != nil {
			return err
		}
		return fmt.Errorf("Http response status code Error: (%d) %s", resp.StatusCode, string(buf))
	}

	//body, err := ioutil.ReadAll(resp.Body)
	body, err1 := ioutil.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if err1 != nil {
		return err1
	}
	return nil
}

//DeleteCKE 向CKE post数据
func DeleteCKE(urls []string) error {
	var err error
	for _, url := range urls {
		err = _deleteCKE(url)
		if err == nil {
			return nil
		}
	}
	return err
}
func _deleteCKE(url string) error {

	req, err := http.NewRequest("DELETE", url, nil)

	if err != nil {
		return err
	}

	connectTimeout := time.Duration(2*1000) * time.Millisecond
	readWriteTimeout := time.Duration(10*1000) * time.Millisecond
	httpClient := &http.Client{
		//禁止重定向
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			Dial:            timeoutDialer(connectTimeout, readWriteTimeout),
		},
	}
	resp, err := httpClient.Do(req)
	defer func() {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}()
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		buf, err := ioutil.ReadAll(io.LimitReader(resp.Body, 32*1024))
		if err != nil {
			return err
		}
		return fmt.Errorf("Http response status code Error: (%d) %s", resp.StatusCode, string(buf))
	}

	//body, err := ioutil.ReadAll(resp.Body)
	_, err1 := ioutil.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if err1 != nil {
		return err1
	}
	return nil
}
