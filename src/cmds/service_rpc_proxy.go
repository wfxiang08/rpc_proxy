package main

import (
	"proxy"
)

const (
	BINARY_NAME  = "rpc_proxy"
	SERVICE_DESC = "Thrift RPC Local Proxy v0.1"
)

var (
	gitVersion string
	buildDate  string
)

func main() {
	proxy.RpcMainProxy(BINARY_NAME, SERVICE_DESC,
		// 验证LB的配置
		proxy.ConfigCheckRpcProxy,
		// 根据配置创建一个Server
		func(config *proxy.ProxyConfig) proxy.Server {
			// 正式的服务
			return proxy.NewProxyServer(config)
		}, buildDate, gitVersion)

}
