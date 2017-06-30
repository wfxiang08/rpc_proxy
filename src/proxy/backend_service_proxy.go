//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	"github.com/wfxiang08/cyutils/utils/atomic2"
	log "github.com/wfxiang08/cyutils/utils/rolling_log"
	"runtime"
	"strings"
	"sync"
	"time"
)

//
// Proxy中用来和后端服务通信的模块
//
type BackService struct {
	productName string
	serviceName string
	topo        *Topology

	// 同时保护: activeConns 和 currentConnIndex
	activeConnsLock  sync.Mutex
	activeConns      []*BackendConn // 每一个BackendConn应该有一定的高可用保障
	currentConnIndex int

	// 用于zk的状态管理(记录当前有效的Conn)
	addr2Conn       map[string]*BackendConn
	verbose         bool
	stop            atomic2.Bool
	lastRequestTime atomic2.Int64
	evtbus          chan interface{}
}

// 创建一个BackService
func NewBackService(productName string, serviceName string, topo *Topology, verbose bool) *BackService {

	service := &BackService{
		productName: productName,
		serviceName: serviceName,
		activeConns: make([]*BackendConn, 0, 10),
		addr2Conn:   make(map[string]*BackendConn),
		topo:        topo,
		verbose:     verbose,
	}

	service.WatchBackServiceNodes()

	go func() {
		for !service.stop.Get() {
			log.Printf(Blue("[Report]: %s --> %d backservice, coroutine: %d"),
				service.serviceName, service.Active(), runtime.NumGoroutine())
			time.Sleep(time.Second * 10)
		}
	}()

	return service

}

func (s *BackService) Stop() {
	// 标志停止
	s.stop.Set(true)
	// 触发一个事件（之后ServiceNodes也不再监控)
	s.evtbus <- true
	go func() {
		// TODO:
		for true {
			now := time.Now().Unix()
			if now-s.lastRequestTime.Get() > 10 {
				break
			} else {
				time.Sleep(time.Second)
			}
		}
		for len(s.activeConns) > 0 {
			s.activeConns[0].MarkOffline()
		}

		log.Printf(Red("Mark All Connections Off: %s"), s.serviceName)

	}()
}

func (s *BackService) Active() int {
	return len(s.activeConns)
}

//
// 如何处理后端服务的变化呢?
//
func (s *BackService) WatchBackServiceNodes() {
	s.evtbus = make(chan interface{}, 2)
	servicePath := s.topo.ProductServicePath(s.serviceName)

	go func() {
		for !s.stop.Get() {
			serviceIds, err := s.topo.WatchChildren(servicePath, s.evtbus)

			if err == nil {
				// 如何监听endpoints的变化呢?
				addressMap := make(map[string]bool, len(serviceIds))

				for _, serviceId := range serviceIds {
					log.Printf(Green("---->Find Endpoint: %s for Service: %s"), serviceId, s.serviceName)
					endpointInfo, err := GetServiceEndpoint(s.topo, s.serviceName, serviceId)

					if err != nil {
						log.ErrorErrorf(err, "Service Endpoint Read Error: %v\n", err)
					} else {

						log.Printf(Green("---->Add endpoint %s To Service %s"),
							endpointInfo.Frontend, s.serviceName)

						if strings.Contains(endpointInfo.Frontend, ":") {
							addressMap[endpointInfo.Frontend] = true
						} else if s.productName == TEST_PRODUCT_NAME {
							// unix domain socket只在测试的时候可以使用(因为不能实现跨机器访问）
							addressMap[endpointInfo.Frontend] = true
						}
					}
				}

				for addr, _ := range addressMap {
					conn, ok := s.addr2Conn[addr]
					if ok && !conn.IsMarkOffline.Get() {
						continue
					} else {
						// 创建新的连接（心跳成功之后就自动加入到 s.activeConns 中
						s.addr2Conn[addr] = NewBackendConn(addr, s, s.serviceName, s.verbose)
					}
				}

				for addr, conn := range s.addr2Conn {
					_, ok := addressMap[addr]
					if !ok {
						conn.MarkOffline()

						// 删除: 然后等待Conn自生自灭
						delete(s.addr2Conn, addr)
					}
				}

				// 等待事件
				<-s.evtbus
			} else {
				log.WarnErrorf(err, "zk read failed: %s", servicePath)
				// 如果读取失败则，则继续等待5s
				time.Sleep(time.Duration(5) * time.Second)
			}

		}
	}()
}

// 获取下一个active状态的BackendConn
func (s *BackService) NextBackendConn() *BackendConn {
	var backSocket *BackendConn

	s.activeConnsLock.Lock()
	defer s.activeConnsLock.Unlock()

	if len(s.activeConns) == 0 {
		backSocket = nil
	} else {
		if s.currentConnIndex >= len(s.activeConns) {
			s.currentConnIndex = 0
		}
		backSocket = s.activeConns[s.currentConnIndex]
		s.currentConnIndex++
	}

	return backSocket
}

//
// 将消息发送到Backend上去
//
func (s *BackService) HandleRequest(req *Request) (err error) {
	// 并发度可能很高
	backendConn := s.NextBackendConn()

	s.lastRequestTime.Set(time.Now().Unix())

	if backendConn == nil {
		// 没有后端服务
		if s.verbose {
			log.Println(Red("No BackSocket Found for service:"), s.serviceName)
		}
		// 从errMsg来构建异常
		errMsg := GetWorkerNotFoundData(req, "BackService")
		req.Response.Data = errMsg
		// XXX: 没有等待，req.Wait.Wait() 直接返回

		return nil
	} else {
		if s.verbose {
			log.Println("SendMessage With: ", backendConn.Addr(), "For Service: ", s.serviceName)
		}
		backendConn.PushBack(req)
		return nil
	}
}

func (s *BackService) StateChanged(conn *BackendConn) {
	//	log.Printf(Cyan("[%s]StateChanged: %s, Index: %d, Count: %d, IsConnActive: %t"),
	//		s.serviceName, conn.addr, conn.Index, len(s.activeConns),
	//		conn.IsConnActive.Get())

	s.activeConnsLock.Lock()
	defer s.activeConnsLock.Unlock()

	if conn.IsConnActive.Get() {
		// 上线: BackendConn
		log.Printf(Cyan("[%s]MarkConnActiveOK: %s, Index: %d, Count: %d"),
			s.serviceName, conn.addr, conn.Index, len(s.activeConns))

		if conn.Index == INVALID_ARRAY_INDEX {
			conn.Index = len(s.activeConns)
			s.activeConns = append(s.activeConns, conn)

			log.Printf(Green("[%s]Add BackendConn to activeConns: %s, Total Actives: %d"),
				s.serviceName, conn.Addr(), len(s.activeConns))
		}
	} else {
		// 下线BackendConn(急速执行
		connIndex := conn.Index
		if conn.Index != INVALID_ARRAY_INDEX {
			lastIndex := len(s.activeConns) - 1

			// 将最后一个元素和当前的元素交换位置
			if lastIndex != conn.Index {

				lastConn := s.activeConns[lastIndex]
				s.activeConns[conn.Index] = lastConn
				lastConn.Index = conn.Index
			}

			s.activeConns[lastIndex] = nil
			conn.Index = INVALID_ARRAY_INDEX

			// slice
			s.activeConns = s.activeConns[0:lastIndex]
			log.Printf(Red("[%s]Remove BackendConn From activeConns: %s, Remains: %d"),
				s.serviceName, conn.Addr(), len(s.activeConns))
		}
		log.Printf(Red("[%s]Remove BackendConn From activeConns: %s, Index: %d"),
			s.serviceName, conn.Addr(), connIndex)
	}
}
