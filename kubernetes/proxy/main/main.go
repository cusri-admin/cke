package main

import (
	"cke/log"
	"os"
	"os/signal"
	"syscall"
	"flag"
	"cke/kubernetes/proxy"
	"net/http"
)

var (
	bindIp string = "0.0.0.0"
	/*proxyListenPort = flag.String("proxy_server_port",	"5443" , "the port used to update cluster ip gateway")*/
	gatewayListenPort = flag.String("gateway_manager_port",	"48080" , "the port used to update cluster ip gateway")
	apiserverListenPort = flag.String("apiserver_proxy_port",	"6443" , "the port used to api server proxy listen")
	dashbaordListenPort = flag.String("dashboard_proxy_port",	"8443" , "the port used to dashboard proxy listen")

	apiServerAddr    = flag.String("apiserver_address", "https://11.254.0.1:443", "apiserver service address <schema>://<svc-cluster-ip>:<port>")
	dashboardAddr   = flag.String("dashboard_address", "https://11.254.0.2:443", "dashbaord service address <schema>://<svc-cluster-ip>:<port>")
	gottyAddr  = flag.String("gotty_address", "http://127.0.0.1:17460", "gotty address <schema>://<svc-cluster-ip>:<port>")

	dashKeyFilePath = flag.String("dashboard_key_file", "/etc/kubernetes/dashboardcrt/dashboard.key", "server key file path")
	dashCrtFilePath = flag.String("dashboard_crt_file", "/etc/kubernetes/dashboardcrt/dashboard.crt", "server crt file path")

	apiKeyFilePath = flag.String("apiserver_key_file", "/etc/kubernetes/ssl/server-key.pem", "server key file path")
	apiCrtFilePath = flag.String("apiserver_crt_file", "/etc/kubernetes/ssl/server.pem", "server crt file path")

	/*serverKeyFilePath = flag.String("proxy_server_key_file", "/etc/kubernetes/ssl/server-key.pem", "server key file path")
	serverCrtFilePath = flag.String("proxy_server_crt_file", "/etc/kubernetes/ssl/server.pem", "server crt file path")*/

	clusterIPRange = flag.String("cluster_ip_range", "11.254.0.0/16", "k8s service clustr ip range")
	svcGateway = flag.String("svc_gateway_node_ip",	"" , "k8s service clustr ip range")
)

func startListGatewayManager(){
	gatewayManager := proxy.NewGatewayManager(svcGateway, clusterIPRange)
	go func() {
		if err := http.ListenAndServe(bindIp + ":" + *gatewayListenPort, gatewayManager); err != nil{
			log.Errorf("start gateway manager http server failed! msg: %s", err.Error())
		}
	}()

}

func main() {
	flag.Parse()
	log.Debugf("all flags: ")
	flag.VisitAll(func(i *flag.Flag) {
		log.Debugf(i.Name + " : " + i.Value.String())
	})

	var apiServerProxy *proxy.CkeKubernetesProxy
	var dashboardProxy *proxy.CkeKubernetesProxy
	var err error
	//apiserver proxy
	if apiServerAddr != nil {
		apiListenAddr := bindIp +":"+ *apiserverListenPort
		if apiServerProxy, err = proxy.NewCkeKubernetesProxy(apiServerAddr, &apiListenAddr, apiKeyFilePath, apiCrtFilePath);err!= nil{
			log.Errorf("create apiserver proxy failed! msg: %s", err.Error())
		}else{
			go func() {
				if err := apiServerProxy.Start(); err != nil{
					log.Errorf("start apiserver proxy failed! msg: %s", err.Error())
				}
			}()
		}
	}

	//dashboard proxy
	if dashboardAddr != nil {
		dashListenAddr := bindIp +":"+  *dashbaordListenPort
		if dashboardProxy, err = proxy.NewCkeKubernetesProxy(dashboardAddr, &dashListenAddr, dashKeyFilePath, dashCrtFilePath);err!= nil{
			log.Errorf("create dashbaord proxy failed! msg: %s", err.Error())
		}else{
			go func() {
				if err := dashboardProxy.Start(); err != nil{
					log.Errorf("start dashboard proxy failed! msg: %s", err.Error())
				}
			}()
		}
	}

	/*//proxy server
	var  proxyServer *proxy.CkeKubernetesProxy
	var err error
    proxyListenAddr := bindIp + ":" + *proxyListenPort
	if proxyServer, err = proxy.NewCkeKubernetesProxy(&proxyListenAddr, serverKeyFilePath, serverCrtFilePath);err!= nil{
		log.Errorf("create proxy server failed! msg: %s", err.Error())
	}else{
		proxyServer.AddBackend("/", apiServerAddr)
		proxyServer.AddBackend("/dashboard", dashboardAddr)
		proxyServer.AddBackend("/console", gottyAddr)
		go func() {
			if err := proxyServer.Start(); err != nil{
				log.Errorf("start proxy server failed! msg: %s", err.Error())
			}
		}()
	}*/

    //svc gateway ip update
	startListGatewayManager()

	//创建监听退出chan
	exitChan := make(chan os.Signal)
	//监听指定信号 ctrl+c kill
	signal.Notify(exitChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM,
		syscall.SIGQUIT, syscall.SIGKILL)
	for s := range exitChan {
		switch s {
		case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGKILL:
			log.Info("Shutdowning Server ...")
			if apiServerProxy != nil {
				apiServerProxy.Stop()
			}
			if dashboardProxy != nil {
				dashboardProxy.Stop()
			}
			os.Exit(0)
		}
	}
	log.Info("Shutdown OK.")
}

