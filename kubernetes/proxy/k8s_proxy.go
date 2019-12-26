package proxy

import (
	"net/http"
	"net/url"
	"time"
	"cke/log"
	"crypto/tls"
	"net"
	"net/http/httputil"
	"github.com/pkg/errors"
	"github.com/gin-gonic/gin"
)

//HTTPServer 保存http server的数据
type CkeKubernetesProxy struct {
	name              *string
	listenAddress     *string
	httpServer 		  *http.Server
	backend           *string
	tlsConfig		  *tls.Config
	serverKeyFile     *string
	serverCrtFile	  *string

	httpRoute *gin.Engine
}


func (cp *CkeKubernetesProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Infof("api server proxy request info [ %+v ]", r)

	remote, err := url.Parse(*cp.backend)
	if err != nil {
		panic(err)
	}
	log.Infof("remote address : %s",remote)
	proxy := httputil.NewSingleHostReverseProxy(remote)
	tlsConfig , err := cp.getTLSConfig()
	if err != nil {
		log.Error( err.Error())
	}
	proxy.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: tlsConfig,
	}

	log.Errorf(">>>>> request.URL: %+v \n request.URI: %s", r.URL, r.RequestURI)

	proxy.ServeHTTP(w, r)
}


func  (cp *CkeKubernetesProxy) getTLSConfig() (*tls.Config, error) {
	if cp.tlsConfig != nil {
		return cp.tlsConfig, nil
	}
	// load cert
	/*cert, err := tls.LoadX509KeyPair("serverCrt", "serverKey")
	if err != nil {
		log.Error("load wechat keys fail", err)
		return nil, err
	}
	//root ca
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(ap.rootCrt)*/
	cp.tlsConfig = &tls.Config{
		//Certificates: []tls.Certificate{cert},
		//RootCAs:      pool,
		InsecureSkipVerify: true,
	}
	return cp.tlsConfig, nil
}

func (cp *CkeKubernetesProxy)Start() error {
	//被代理的服务器host和port
	log.Infof("start proxy to backend %s",*cp.backend)

	cp.httpServer = &http.Server{
		Addr: *cp.listenAddress,
		Handler: cp,
	}

	return  cp.httpServer.ListenAndServeTLS(*cp.serverCrtFile, *cp.serverKeyFile)
	//return  http.ListenAndServe(*ap.listenAddress, ap)
}

func (cp *CkeKubernetesProxy)Stop() error {
	//被代理的服务器host和port
	log.Infof("stop proxy to backend %s",*cp.backend)
	cp.httpServer.Close();
	return  http.ListenAndServeTLS(*cp.listenAddress, "serverCrt","serverKey", cp)
	//return  http.ListenAndServe(*ap.listenAddress, ap)
}

func NewCkeKubernetesProxy(backend *string, listenAddress *string, serverKeyFile *string, serverCrtFile *string) (*CkeKubernetesProxy, error){
	if backend == nil {
		return nil, errors.New("Could not find backend address")
	}
	return &CkeKubernetesProxy{
		listenAddress: listenAddress,
		backend: backend,
		serverKeyFile: serverKeyFile,
		serverCrtFile: serverCrtFile,
		httpRoute: gin.Default(),
	}, nil
}





