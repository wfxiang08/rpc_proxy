//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

//import (
//	"fmt"
//	"utils/assert"

//	"net"
//	"testing"
//)

//func TestZooKeeper(t *testing.T) {

//	ifaces, _ := net.Interfaces()
//	// handle err
//	for _, i := range ifaces {
//		addrs, _ := i.Addrs()
//		// handle err
//		for _, addr := range addrs {
//			var ip net.IP
//			switch v := addr.(type) {
//			case *net.IPNet:
//				ip = v.IP
//			case *net.IPAddr:
//				ip = v.IP
//			}
//			fmt.Println("IP: ", ip)
//			// process IP address
//		}
//	}

//	// 创建一个Topology
//	top := NewTopology("online_service", "127.0.0.1:2181")

//	testPath := "/hello"
//	testPath = top.FullPath(testPath)
//	top.DeleteDir(testPath)
//	path, err := top.CreateDir(testPath)
//	fmt.Println("Full Path: ", path)
//	fmt.Println("Error: ", err)
//	assert.Must(true)

//	var proxyInfo map[string]interface{} = make(map[string]interface{})

//	proxyInfo["rpc_front"] = "tcp://127.0.0.1:5550"

//	fmt.Println("ProxyInfo: ", proxyInfo)

//	top.SetRpcProxyData(proxyInfo)
//	data, err := top.GetRpcProxyData()
//	fmt.Println("Data: ", data, ", error: ", err)

//	//	var endpointInfo map[string]interface{} = make(map[string]interface{})

//	//	endpointInfo["endpoint"] = "tcp://127.0.0.1:5555"
//	//	top.AddServiceEndPoint("account", "server001", endpointInfo)
//	top.DeleteServiceEndPoint("account", "server001")

//	//	top.SetPathData(testPath, []byte("hello"))
//	//	var wait chan int32 = make(chan int32, 4)

//	//	go func() {
//	//		var evtbus chan interface{} = make(chan interface{})
//	//		var pathes []string
//	//		for i := 0; i < 10; i++ {
//	//			pathes, _ = top.WatchChildren(testPath, evtbus)
//	//			fmt.Println("pathes: ", pathes)
//	//			<-evtbus
//	//		}

//	//		wait <- 1

//	//		pathes, _ = top.WatchChildren(testPath, evtbus)
//	//		fmt.Println("pathes: ", pathes)
//	//		wait <- 2
//	//	}()

//	//	go func() {
//	//		for i := 0; i < 10; i++ {
//	//			path := fmt.Sprintf("%s/node_%d", testPath, i)
//	//			top.CreateDir(path)
//	//		}

//	//		wait <- 100
//	//	}()

//	//	go func() {
//	//		var evtbus chan interface{} = make(chan interface{})
//	//		content, _ := top.WatchNode(testPath, evtbus)
//	//		fmt.Println("--------Content: ", string(content))
//	//		fmt.Println("--------EvtBus: ", <-evtbus)

//	//		content, _ = top.WatchNode(testPath, evtbus)
//	//		fmt.Println("--------Content: ", string(content))
//	//		wait <- 1234
//	//	}()

//	//	go func() {
//	//		top.SetPathData(testPath, []byte("world"))
//	//		wait <- 5678

//	//		time.Sleep(100 * time.Millisecond)

//	//		top.SetPathData(testPath, []byte("world"))
//	//		wait <- 0

//	//	}()

//	//	for i := 0; i < 3; i++ {
//	//		fmt.Println("Waiting: ", <-wait)
//	//	}
//}
