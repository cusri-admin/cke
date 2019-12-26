package rsa

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io/ioutil"
	"math/big"
	rd "math/rand"
	"os"
	"time"
	"net"
	"log"
)

func init() {
	rd.Seed(time.Now().UnixNano())
}

type CertInformation struct {
	Country            []string
	Organization       []string
	OrganizationalUnit []string
	EmailAddress       []string
	Province           []string
	Locality           []string
	CommonName         string
	CrtName			   string
	KeyName            string
	IsCA               bool
	Names              []pkix.AttributeTypeAndValue
	IPAddresses        []net.IP
	DNSNames		   []string
}

func createCRT(RootCa *x509.Certificate, RootKey *rsa.PrivateKey, info *CertInformation, isWriteFile bool) (crtPemBytes []byte,privateKeyPemBytes []byte, err error){
	Crt := newCertificate(info)
	Key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil,nil,err
	}

	var buf []byte
	if RootCa == nil || RootKey == nil {
		//创建自签名证书
		buf, err = x509.CreateCertificate(rand.Reader, Crt, Crt, &Key.PublicKey, Key)
	} else {
		//使用根证书签名
		buf, err = x509.CreateCertificate(rand.Reader, Crt, RootCa, &Key.PublicKey, RootKey)
	}
	if err != nil {
		return  nil,nil,err
	}
    if isWriteFile {
		err = write(info.CrtName, "CERTIFICATE", buf)
		if err != nil {
			return   nil,nil,err
		}
	}
	crtPemBytes = pem.EncodeToMemory(&pem.Block{Type:"CERTIFICATE", Bytes: buf})

	buf = x509.MarshalPKCS1PrivateKey(Key)
	if isWriteFile {
		err = write(info.KeyName, "RSA PRIVATE KEY", buf)
		if err != nil {
			return nil,nil,err
		}
	}
	privateKeyPemBytes = pem.EncodeToMemory(&pem.Block{Type:"RSA PRIVATE KEY", Bytes: buf})

	return crtPemBytes, privateKeyPemBytes,nil
}
//编码写入文件
func write(filename, Type string, p []byte) error {
	File, err := os.Create(filename)
	defer File.Close()
	if err != nil {
		return err
	}
	var b *pem.Block = &pem.Block{Bytes: p, Type: Type}
	return pem.Encode(File, b)
}

func Parse(crtPath, keyPath string) (rootcertificate *x509.Certificate, rootPrivateKey *rsa.PrivateKey, err error) {
	rootcertificate, err = ParseCrtFromFile(crtPath)
	if err != nil {
		return
	}
	rootPrivateKey, err = ParseKeyFromFile(keyPath)
	return
}

func ParseCrtFromFile(path string) (*x509.Certificate, error) {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	p := &pem.Block{}
	p, buf = pem.Decode(buf)
	return x509.ParseCertificate(p.Bytes)
}

func ParseCrtFromPemBytes(pemCrtBytes []byte) (*x509.Certificate, error) {
	p , _ := pem.Decode(pemCrtBytes)
	return x509.ParseCertificate(p.Bytes)
}

func ParseKeyFromFile(path string) (*rsa.PrivateKey, error) {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	p, buf := pem.Decode(buf)
	return x509.ParsePKCS1PrivateKey(p.Bytes)
}

func ParseKeyFromPemBytes(keyPemBytes []byte) (*rsa.PrivateKey, error) {
	p,_ := pem.Decode(keyPemBytes)
	return x509.ParsePKCS1PrivateKey(p.Bytes)
}

func newCertificate(info *CertInformation) *x509.Certificate {
	return &x509.Certificate{
		SerialNumber: big.NewInt(rd.Int63()),
		Subject: pkix.Name{
			Country:            info.Country,
			Organization:       info.Organization,
			OrganizationalUnit: info.OrganizationalUnit,
			Province:           info.Province,
			CommonName:         info.CommonName,
			Locality:           info.Locality,
			Names:         		info.Names,
		},
		NotBefore:             time.Now(),//证书的开始时间
		NotAfter:              time.Now().AddDate(20, 0, 0),//证书的结束时间
		BasicConstraintsValid: true, //基本的有效性约束
		IsCA:           info.IsCA,   //是否是根证书
		ExtKeyUsage:    []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},    //证书用途
		KeyUsage:       x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign | x509.KeyUsageKeyEncipherment,
		EmailAddresses: info.EmailAddress,
		IPAddresses: info.IPAddresses,
		DNSNames: info.DNSNames,
	}
}

func createBaseInfo() (*CertInformation){
	return &CertInformation{Country: []string{"CN"}, Organization: []string{"CUS"}, IsCA: true,
		OrganizationalUnit: []string{"JG"}, EmailAddress: []string{"wanglp194@chinaunicom.cn"},
		Locality: []string{"Bejing"}, Province: []string{"Bejing"}, CommonName: "kubernetes",
		CrtName: "ca.crt", KeyName: "ca.key"}
}

func CreateRootCrtAndKey(isWriteFile bool) ([]byte,[]byte,error){
	baseInfo := createBaseInfo()
	baseInfo.CommonName ="kubernetes"
	crtPemBytes, priKeyPemBytes, err := createCRT(nil, nil, baseInfo, isWriteFile)
	if err != nil {
		return nil,nil,err
	}
	return crtPemBytes, priKeyPemBytes,nil
}

func SignCrtAndKey(rootCrtPemBytes []byte, rootPriKeyPemBytes []byte, name string ,commonName string, ipAddress []string,
	dnsNames []string, isWriteFile bool) ([]byte,[]byte,error) {
	crtInfo := createBaseInfo()
	crtInfo.CommonName =commonName
	crtInfo.IsCA = false
	crtInfo.CrtName = name+".crt"
	crtInfo.KeyName = name+".key"
	for _,ipAdd := range ipAddress{
		crtInfo.IPAddresses = append(crtInfo.IPAddresses, net.ParseIP(ipAdd))
	}
	for _,dns := range dnsNames{
		crtInfo.DNSNames = append(crtInfo.DNSNames, dns)
	}


	crt, err:= ParseCrtFromPemBytes(rootCrtPemBytes)
	if err != nil {
		log.Println("Parse crt error,Error info:", err)
		return nil,nil,err
	}
	pri, err:= ParseKeyFromPemBytes(rootPriKeyPemBytes)
	if err != nil {
		log.Println("Parse crt error,Error info:", err)
		return nil,nil,err
	}
	crtPemBytes , priKeyPemBytes, err := createCRT(crt, pri, crtInfo, isWriteFile)
	if err != nil {
		log.Println("Create crt error,Error info:", err)
	}
	return crtPemBytes , priKeyPemBytes, nil
}

func Test_crt() {
	baseInfo := createBaseInfo()
	crtPemBytes, priKeyPemBytes, err := createCRT(nil, nil, baseInfo, true)
	if err != nil {
		log.Println("Create crt error,Error info:", err)
		return
	}
	crtInfo := createBaseInfo()
	crtInfo.IsCA = false
	crtInfo.CrtName = "kubernetes.crt"
	crtInfo.KeyName = "kubernetes.key"
	crtInfo.IPAddresses = append(crtInfo.IPAddresses, net.ParseIP("10.124.220.11"),
		net.ParseIP("10.124.220.12"), net.ParseIP("10.124.220.13"))
	//crtInfo.Names = []pkix.AttributeTypeAndValue{{asn1.ObjectIdentifier{2, 1, 3}, "MAC_ADDR"}} //添加扩展字段用来做自定义使用
	//crt, pri, err := Parse(baseInfo.CrtName, baseInfo.KeyName)
	crt, err:= ParseCrtFromPemBytes(crtPemBytes)
	if err != nil {
		log.Println("Parse crt error,Error info:", err)
		return
	}
	pri, err:= ParseKeyFromPemBytes(priKeyPemBytes)
	if err != nil {
		log.Println("Parse crt error,Error info:", err)
		return
	}
	crtPemBytes , priKeyPemBytes, err = createCRT(crt, pri, crtInfo,true)
	if err != nil {
		log.Println("Create crt error,Error info:", err)
	}
	println(string(crtPemBytes))
	println(string(priKeyPemBytes))
	//os.Remove(baseInfo.CrtName)
	//os.Remove(baseInfo.KeyName)
	//os.Remove(crtInfo.CrtName)
	//os.Remove(crtInfo.KeyName)
}