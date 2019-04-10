package utils

import (
	"fmt"
	"github.com/urfave/cli"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var (
	//NetProxyPort NetProxyPort
	NetProxyPort = cli.IntFlag{
		Name:  "proxyport, t",
		Usage: "http proxy port",
		Value: 65533,
	}
	//HTTPLocalAddr LocalAddr
	HTTPLocalAddr = cli.StringFlag{
		Name:  "proxyaddr",
		Usage: "http proxy addr",
		Value: "127.0.0.1",
	}
)

//AddHTTPProxyFlags Add http proxy flags
func AddHTTPProxyFlags() []cli.Flag {
	return []cli.Flag{
		NetProxyPort,
		HTTPLocalAddr,
	}
}

//ProxyConfig ProxyConfig
type ProxyConfig struct {
	NetProxyPort int           `yaml:"net_proxy_port"`
	Addr         string         `yaml:"addr"`
	Version      string         `yaml:"version"`
	Signal       chan os.Signal `yaml:"-" json:"-"`
	Procotol     string         `yaml:"procotol"`
}

//DefaultProxyConfig DefaultProxyConfig
var DefaultProxyConfig = &ProxyConfig{
	NetProxyPort: 65532,
	Addr:         "127.0.0.1",
	Version:      "1.0.0",
	Procotol:     "http",
}

//NewHTTPProxyPlugin NewPlugin
func NewHTTPProxyPlugin(ctx *cli.Context) PluginHandlerInterface {
	conf := &ProxyConfig{
		NetProxyPort: ctx.Int("proxyport"),
		Addr:         "127.0.0.1", //固定使用127 安保
		Version:      "1.0.0",
		Procotol:     "http",
		Signal:       make(chan os.Signal, 1),
	}
	return conf
}

//TroyRun Net proxy run
func (pc *ProxyConfig) TroyRun() {
	s := NewSocketHandler(pc)
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", pc.Addr, pc.NetProxyPort),
		Handler:      s,
		ReadTimeout:  1 * time.Hour,
		WriteTimeout: 1 * time.Hour,
	}
	log.Printf("Start netproxy server, listen: %s\n", server.Addr)
	// go func() {
	// 	log.Println("Listen http")
	// 	err := server.ListenAndServe()
	// 	if err != nil {
	// 		log.Println("HTTP proxy with error:", err.Error())
	// 	}
	// 	log.Panicf("server listen error : %s", err.Error())
	// }()
	log.Println("Listen http")
	err := server.ListenAndServe()
	if err != nil {
		log.Println("HTTP proxy with error:", err.Error())
	}
	log.Panicf("server listen error : %s", err.Error())
	select {
	case s := <-pc.Signal:
		fmt.Printf("Http Proxy got exit signal: %s...\nBye\n", s.String())
	}
}

//TroyExit TroyExit
func (pc *ProxyConfig) TroyExit() {
	signal.Notify(pc.Signal, os.Interrupt, syscall.SIGTERM)
}

//GetPluginAddrs get plugins server addrs
func (pc *ProxyConfig) GetPluginAddrs() string {
	return fmt.Sprintf("%s:%d", pc.Addr, pc.NetProxyPort)
}

//GetPluginProcotol GetPluginProcotol
func (pc *ProxyConfig) GetPluginProcotol() string {
	return pc.Procotol
}

//SocketHandler SocketHandler
type SocketHandler struct {
	PC        *ProxyConfig
	Mutex     sync.Mutex
	TransAddr string
}

//NewSocketHandler NewSocketHandler
func NewSocketHandler(pc *ProxyConfig) *SocketHandler {
	s := new(SocketHandler)
	s.PC = pc
	return s
}

//ServerHTTP 实现serverHTTP接口
func (s *SocketHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	connIn, _, err := resp.(http.Hijacker).Hijack()
	if err != nil {
		log.Printf("hijack error: %s", err.Error())
		return
	}
	address, err := dealHost(req.URL.Host)
	if err != nil {
		log.Printf("parse url error: %s\n", err.Error())
		return
	}
	connOut, err := net.Dial("tcp", address)
	if err != nil {
		log.Println("dial tcp error", err)
	}
	if req.Method == "CONNECT" {
		log.Println("https connections")
		b := []byte("HTTP/1.1 200 Connection Established\r\n" +
			"Proxy-Agent: kerrigan:" + s.PC.Version + "\r\n" +
			"Content-Length: 0" + "\r\n\r\n")
		_, err := connIn.Write(b)
		if err != nil {
			log.Println("Write Connect err:", err)
			return
		}
	} else {
		log.Println("http connections")
		req.Header.Del("Proxy-Connection")
		req.Header.Set("Connection", "Keep-Alive")
		err := req.Write(connOut)
		if err != nil {
			log.Println("send to server err", err)
			return
		}
	}
	err = Transport(connIn, connOut)
	if err != nil {
		log.Println("trans error ", err)
	}
}

func dealHost(host string) (string, error) {
	hostPortURL, err := url.Parse(host)
	if err != nil {
		fmt.Printf("parse url error: %s\n", err.Error())
		return "", err
	}
	fmt.Println(hostPortURL.Opaque)
	var address string
	if hostPortURL.Opaque != "" {
		address = hostPortURL.Scheme + ":" + hostPortURL.Opaque
	} else {
		address = host + ":80"
	}
	return address, nil
}

//Transport tunnel trans port data
func Transport(conn1, conn2 net.Conn) (err error) {
	rChan := make(chan error, 1)
	wChan := make(chan error, 1)
	
	go StrCopy(conn1, conn2, wChan)
	go StrCopy(conn2, conn1, rChan)
	select {
	case err = <-wChan:
	case err = <-rChan:
	}
	return
}

//StrCopy StrCopy
func StrCopy(src io.Reader, dst io.Writer, ch chan<- error) {
	_, err := io.Copy(dst, src)
	ch <- err
}
