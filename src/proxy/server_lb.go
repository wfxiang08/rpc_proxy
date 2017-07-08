//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	"fmt"
	"github.com/wfxiang08/cyutils/utils/atomic2"
	log "github.com/wfxiang08/cyutils/utils/rolling_log"
	"github.com/wfxiang08/thrift_rpc_base/rpc_utils"
	thrift "github.com/wfxiang08/go_thrift/thrift"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

//
// Thrift Server的参数
//
type ThriftLoadBalanceServer struct {
	productName     string
	serviceName     string
	frontendAddr    string // 绑定的端口
	backendAddr     string
	lbServiceName   string
	topo            *Topology // ZK相关
	zkAddr          string
	verbose         bool
	backendService  *BackServiceLB
	exitEvt         chan bool
	lastRequestTime atomic2.Int64
	config          *ServiceConfig
}

func NewThriftLoadBalanceServer(config *ServiceConfig) *ThriftLoadBalanceServer {
	log.Printf("FrontAddr: %s\n", Magenta(config.FrontendAddr))

	// 前端对接rpc_proxy
	p := &ThriftLoadBalanceServer{
		config:       config,
		zkAddr:       config.ZkAddr,
		productName:  config.ProductName,
		serviceName:  config.Service,
		frontendAddr: config.FrontendAddr,
		backendAddr:  config.BackAddr,
		verbose:      config.Verbose,
		exitEvt:      make(chan bool),
	}

	p.topo = NewTopology(p.productName, p.zkAddr)
	p.lbServiceName = GetServiceIdentity(p.frontendAddr)

	// 后端对接: 各种python的rpc server
	p.backendService = NewBackServiceLB(p.serviceName, p.backendAddr, p.verbose,
		p.config.FalconClient, p.exitEvt)
	return p

}

func (p *ThriftLoadBalanceServer) Run() {
	//	// 1. 创建到zk的连接

	// 127.0.0.1:5555 --> 127_0_0_1:5555

	exitSignal := make(chan os.Signal, 1)

	signal.Notify(exitSignal, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)
	// syscall.SIGKILL
	// kill -9 pid
	// kill -s SIGKILL pid 还是留给运维吧
	//

	// 注册服务
	evtExit := make(chan interface{})

	// 初始状态为不上线
	var state atomic2.Bool
	state.Set(false)
	stateChan := make(chan bool)

	serviceEndpoint := RegisterService(p.serviceName, p.frontendAddr, p.lbServiceName,
		p.topo, evtExit, p.config.WorkDir, p.config.CodeUrlVersion, &state, stateChan)

	//	var suideTime time.Time

	//	isAlive := true

	// 3. 读取后端服务的配置
	var transport thrift.TServerTransport
	var err error

	isUnixDomain := false
	// 127.0.0.1:9999(以:区分不同的类型)
	if !strings.Contains(p.frontendAddr, ":") {
		if rpc_utils.FileExist(p.frontendAddr) {
			os.Remove(p.frontendAddr)
		}
		transport, err = rpc_utils.NewTServerUnixDomain(p.frontendAddr)
		isUnixDomain = true
	} else {
		transport, err = thrift.NewTServerSocket(p.frontendAddr)
	}

	if err != nil {
		log.ErrorErrorf(err, "Server Socket Create Failed: %v", err)
		panic(fmt.Sprintf("Invalid FrontendAddress: %s", p.frontendAddr))
	}

	err = transport.Listen()
	if err != nil {
		log.ErrorErrorf(err, "Server Socket Create Failed: %v", err)
		panic(fmt.Sprintf("Binding Error FrontendAddress: %s", p.frontendAddr))
	}

	ch := make(chan thrift.TTransport, 4096)
	defer close(ch)

	// 等待后端服务起来
	waitTicker := time.NewTicker(time.Second)

	// 等待上线采用的策略:
	// 1. 检测到有效的Worker注册之后，再等5s即可像zk注册; 避免了Worker没有连接上来，就有请求过来
	// 2. 一旦注册之后，就不再使用该策略；避免服务故障时，lb频繁更新zk, 导致proxy等频繁读取zk
START_WAIT:
	for true {
		select {
		case <-waitTicker.C:
			if p.backendService.Active() <= 0 {
				log.Infof("Sleep Waiting for back Service to Start")
				time.Sleep(time.Second)
			} else {
				break START_WAIT
			}
		case <-exitSignal:
			// 直接退出
			transport.Interrupt()
			transport.Close()
			return
		}
	}

	log.Infof("Stop Waiting")
	// 停止: waitTicker, 再等等就继续了
	waitTicker.Stop()
	time.Sleep(time.Second * 5)

	log.Infof("Begin to Reg To Zk...")
	state.Set(true)
	stateChan <- true

	// 强制退出? TODO: Graceful退出
	go func() {
		<-exitSignal

		// 通知RegisterService终止循环
		evtExit <- true
		log.Info(Green("Receive Exit Signals...."))
		serviceEndpoint.DeleteServiceEndpoint(p.topo)

		start := time.Now().Unix()
		for true {
			// 如果5s内没有接受到新的请求了，则退出
			now := time.Now().Unix()
			if now-p.lastRequestTime.Get() > 5 {
				log.Printf(Red("[%s]Graceful Exit..."), p.serviceName)
				break
			} else {
				log.Printf(Cyan("[%s]Sleeping %d seconds before Exit...\n"),
					p.serviceName, now-start)
				time.Sleep(time.Second)
			}
		}

		transport.Interrupt()
		transport.Close()
	}()

	go func() {
		var address string
		for c := range ch {
			// 为每个Connection建立一个Session
			socket, ok := c.(rpc_utils.SocketAddr)

			if ok {
				if isUnixDomain {
					address = p.frontendAddr
				} else {
					address = socket.Addr().String()
				}
			} else {
				address = "unknow"
			}
			x := NewNonBlockSession(c, address, p.verbose, &p.lastRequestTime)
			// Session独立处理自己的请求
			go x.Serve(p.backendService, 1000)
		}
	}()

	// Accept什么时候出错，出错之后如何处理呢?
	for {
		c, err := transport.Accept()
		if err != nil {
			close(ch)
			break
		} else {
			ch <- c
		}
	}
}
