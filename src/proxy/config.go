//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"fmt"
	"github.com/c4pt0r/cfg"
	log "github.com/wfxiang08/cyutils/utils/rolling_log"
	"os"
	"path"
	"strings"
)

type ProductConfig struct {
	ProductName      string
	ZkAddr           string
	ZkSessionTimeout int
}
type ServiceConfig struct {
	ProductConfig
	StandAlone     bool
	Service        string
	FrontHost      string
	FrontPort      string
	FrontSock      string
	FrontendAddr   string
	IpPrefix       string

	BackAddr       string

	Profile        bool
	Verbose        bool

	// 提供给dashboard来查看服务状态
	WorkDir        string
	CodeUrlVersion string

	// 用于监控
	FalconClient   string
}

type ProxyConfig struct {
	ProductConfig

	ProxyAddr string
	Profile   bool
	Verbose   bool
}

//
// 通过参数依赖，保证getFrontendAddr的调用位置（必须等待Host, IpPrefix, Port读取完毕之后)
//
func (conf *ServiceConfig) getFrontendAddr(frontHost, ipPrefix, frontPort string) string {
	if conf.FrontSock != "" {
		return conf.FrontSock
	}

	var frontendAddr = ""
	// 如果没有指定FrontHost, 则自动根据 IpPrefix来进行筛选，
	// 例如: IpPrefix: 10., 那么最终内网IP： 10.4.10.2之类的被选中
	if frontHost == "" {
		log.Println("FrontHost: ", frontHost, ", Prefix: ", ipPrefix)
		if ipPrefix != "" {
			frontHost = GetIpWithPrefix(ipPrefix)
		}
	}
	if frontPort != "" && frontHost != "" {
		frontendAddr = fmt.Sprintf("%s:%s", frontHost, frontPort)
	}
	return frontendAddr
}

func LoadConf(configFile string) (*ServiceConfig, error) {
	c := cfg.NewCfg(configFile)
	if err := c.Load(); err != nil {
		log.PanicErrorf(err, "load config '%s' failed", configFile)
	}

	conf := &ServiceConfig{}

	// 读取product
	conf.ProductName, _ = c.ReadString("product", "test")
	if len(conf.ProductName) == 0 {
		log.Panicf("invalid config: product entry is missing in %s", configFile)
	}

	// 读取zk
	conf.ZkAddr, _ = c.ReadString("zk", "")
	if len(conf.ZkAddr) == 0 {
		log.Panicf("invalid config: need zk entry is missing in %s", configFile)
	}
	conf.ZkAddr = strings.TrimSpace(conf.ZkAddr)

	loadConfInt := func(entry string, defInt int) int {
		v, _ := c.ReadInt(entry, defInt)
		if v < 0 {
			log.Panicf("invalid config: read %s = %d", entry, v)
		}
		return v
	}

	conf.ZkSessionTimeout = loadConfInt("zk_session_timeout", 30)
	conf.Verbose = loadConfInt("verbose", 0) == 1

	// 是否独立于zookeeper独立运行
	conf.StandAlone = loadConfInt("stand_alone", 0) == 1

	conf.Service, _ = c.ReadString("service", "")
	conf.Service = strings.TrimSpace(conf.Service)

	conf.FrontHost, _ = c.ReadString("front_host", "")
	conf.FrontHost = strings.TrimSpace(conf.FrontHost)

	conf.FrontPort, _ = c.ReadString("front_port", "")
	conf.FrontPort = strings.TrimSpace(conf.FrontPort)

	conf.FrontSock, _ = c.ReadString("front_sock", "")
	conf.FrontSock = strings.TrimSpace(conf.FrontSock)
	// 配置文件中使用的是相对路径，在注册到zk时，需要还原成为绝对路径
	if len(conf.FrontSock) > 0 && !strings.HasPrefix(conf.FrontSock, "/") {
		dir, _ := os.Getwd()
		conf.FrontSock = path.Clean(path.Join(dir, conf.FrontSock))
	}

	conf.IpPrefix, _ = c.ReadString("ip_prefix", "")
	conf.IpPrefix = strings.TrimSpace(conf.IpPrefix)

	// 注意先后顺序:
	// FrontHost, FrontPort, IpPrefix之后才能计算FrontendAddr
	conf.FrontendAddr = conf.getFrontendAddr(conf.FrontHost, conf.IpPrefix, conf.FrontPort)

	conf.BackAddr, _ = c.ReadString("back_address", "")
	conf.BackAddr = strings.TrimSpace(conf.BackAddr)

	conf.FalconClient, _ = c.ReadString("falcon_client", "")

	profile, _ := c.ReadInt("profile", 0)
	conf.Profile = profile == 1
	return conf, nil
}

func LoadProxyConf(configFile string) (*ProxyConfig, error) {
	c := cfg.NewCfg(configFile)
	if err := c.Load(); err != nil {
		log.PanicErrorf(err, "load config '%s' failed", configFile)
	}

	conf := &ProxyConfig{}

	// 读取product
	conf.ProductName, _ = c.ReadString("product", "test")
	if len(conf.ProductName) == 0 {
		log.Panicf("invalid config: product entry is missing in %s", configFile)
	}

	// 读取zk
	conf.ZkAddr, _ = c.ReadString("zk", "")
	if len(conf.ZkAddr) == 0 {
		log.Panicf("invalid config: need zk entry is missing in %s", configFile)
	}
	conf.ZkAddr = strings.TrimSpace(conf.ZkAddr)

	loadConfInt := func(entry string, defInt int) int {
		v, _ := c.ReadInt(entry, defInt)
		if v < 0 {
			log.Panicf("invalid config: read %s = %d", entry, v)
		}
		return v
	}

	conf.ZkSessionTimeout = loadConfInt("zk_session_timeout", 30)
	conf.Verbose = loadConfInt("verbose", 0) == 1

	conf.ProxyAddr, _ = c.ReadString("proxy_address", "")
	conf.ProxyAddr = strings.TrimSpace(conf.ProxyAddr)

	profile, _ := c.ReadInt("profile", 0)
	conf.Profile = profile == 1
	return conf, nil
}
