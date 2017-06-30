//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	log "github.com/wfxiang08/cyutils/utils/rolling_log"
	"sync"
	"time"
)

type Router struct {
	productName string

	// 只用于保护: services
	serviceLock sync.RWMutex
	services    map[string]*BackService

	topo    *Topology
	verbose bool
}

func NewRouter(productName string, topo *Topology, verbose bool) *Router {
	r := &Router{
		productName: productName,
		services:    make(map[string]*BackService),
		topo:        topo,
		verbose:     verbose,
	}

	// 监控服务的变化
	r.WatchServices()

	return r
}

//
// 后端如何处理一个Request
//
func (s *Router) Dispatch(r *Request) error {
	backService := s.GetBackService(r.Service)
	if backService == nil {
		log.Printf(Cyan("Service Not Found for: %s.%s\n"), r.Service, r.Request.Name)
		r.Response.Data = GetServiceNotFoundData(r)
		return nil
	} else {
		return backService.HandleRequest(r)
	}
}

// Router负责监听zk中服务列表的变化
func (bk *Router) WatchServices() {
	var evtbus chan interface{} = make(chan interface{}, 2)

	// 1. 保证Service目录存在，否则会报错
	servicesPath := bk.topo.ProductServicesPath()
	_, e1 := bk.topo.CreateDir(servicesPath)
	if e1 != nil {
		log.PanicErrorf(e1, "Zk Path Create Failed: %s", servicesPath)
	}

	go func() {
		for true {
			// 无限监听
			services, err := bk.topo.WatchChildren(servicesPath, evtbus)

			if err == nil {
				bk.serviceLock.Lock()
				// 保证数据更新是有效的
				oldServices := bk.services
				bk.services = make(map[string]*BackService, len(services))
				for _, service := range services {
					log.Println("Found Service: ", Magenta(service))

					back, ok := oldServices[service]
					if ok {
						bk.services[service] = back
						delete(oldServices, service)
					} else {

						bk.addBackService(service)
					}
				}
				if len(oldServices) > 0 {
					for _, conn := range oldServices {
						// 标记下线(现在应该不会有新的请求，最多只会处理一些收尾的工作
						conn.Stop()
					}

				}

				bk.serviceLock.Unlock()

				// 等待事件
				<-evtbus
			} else {
				log.ErrorErrorf(err, "zk watch error: %s, error: %v\n",
					servicesPath, err)
				time.Sleep(time.Duration(5) * time.Second)
			}
		}
	}()

	// 读取zk, 等待
	log.Println("ProductName: ", Magenta(bk.topo.ProductName))
}

// 添加一个后台服务(非线程安全)
func (bk *Router) addBackService(service string) {

	backService, ok := bk.services[service]
	if !ok {
		backService = NewBackService(bk.productName, service, bk.topo, bk.verbose)
		bk.services[service] = backService
	}

}
func (bk *Router) GetBackService(service string) *BackService {
	bk.serviceLock.RLock()
	backService, ok := bk.services[service]
	bk.serviceLock.RUnlock()

	if ok {
		return backService
	} else {
		return nil
	}
}
