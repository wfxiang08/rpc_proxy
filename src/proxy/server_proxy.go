//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	"fmt"
	log "github.com/wfxiang08/cyutils/utils/rolling_log"
	"github.com/wfxiang08/go_thrift/thrift"
	"github.com/wfxiang08/thrift_rpc_base/rpc_utils"
	"os"
	"strings"
	"time"
)

const (
	HEARTBEAT_INTERVAL = 1000 * time.Millisecond
)

type ProxyServer struct {
	productName string
	proxyAddr   string
	zkAdresses  string
	topo        *Topology
	verbose     bool
	profile     bool
	router      *Router
}

func NewProxyServer(config *Config) *ProxyServer {
	p := &ProxyServer{
		productName: config.ProductName,
		proxyAddr:   config.ProxyAddr,
		zkAdresses:  config.ZkAddr,
		verbose:     config.Verbose,
		profile:     config.Profile,
	}
	p.topo = NewTopology(p.productName, p.zkAdresses)
	p.router = NewRouter(p.productName, p.topo, p.verbose)
	return p
}

//
// 两参数是必须的:  ProductName, zkAddress, frontAddr可以用来测试
//
func (p *ProxyServer) Run() {

	var transport thrift.TServerTransport
	var err error

	log.Printf(Magenta("Start Proxy at Address: %s"), p.proxyAddr)
	// 读取后端服务的配置
	isUnixDomain := false
	if !strings.Contains(p.proxyAddr, ":") {
		if rpc_utils.FileExist(p.proxyAddr) {
			os.Remove(p.proxyAddr)
		}
		transport, err = rpc_utils.NewTServerUnixDomain(p.proxyAddr)
		isUnixDomain = true
	} else {
		transport, err = thrift.NewTServerSocket(p.proxyAddr)
	}
	if err != nil {
		log.ErrorErrorf(err, "Server Socket Create Failed: %v, Front: %s", err, p.proxyAddr)
	}

	// 开始监听
	//	transport.Open()
	transport.Listen()

	ch := make(chan thrift.TTransport, 4096)
	defer close(ch)
	defer func() {
		log.Infof(Red("==> Exit rpc_proxy"))
		if err := recover(); err != nil {
			log.Infof("Error rpc_proxy: %s", err)
		}
	}()
	go func() {
		var address string
		for c := range ch {
			// 为每个Connection建立一个Session
			socket, ok := c.(rpc_utils.SocketAddr)
			if isUnixDomain {
				address = p.proxyAddr
			} else if ok {
				address = socket.Addr().String()
			} else {
				address = "unknow"
			}
			x := NewSession(c, address, p.verbose)
			// Session独立处理自己的请求
			go x.Serve(p.router, 1000)
		}
	}()

	// Accept什么时候出错，出错之后如何处理呢?
	for {
		c, err := transport.Accept()
		if err != nil {
			log.ErrorErrorf(err, "Accept Error: %v", err)
			break
		} else {
			ch <- c
		}
	}
}

func printList(msgs []string) string {
	results := make([]string, 0, len(msgs))
	results = append(results, fmt.Sprintf("Msgs Len: %d, ", len(msgs)-1))
	for i := 0; i < len(msgs)-1; i++ {
		results = append(results, fmt.Sprintf("[%s]", msgs[i]))
	}
	return strings.Join(results, ",")
}
