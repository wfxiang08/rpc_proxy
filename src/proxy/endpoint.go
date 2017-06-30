//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	"encoding/json"
	log "github.com/wfxiang08/cyutils/utils/rolling_log"
	zookeeper "github.com/wfxiang08/go-zookeeper/zk"
	"os"
	os_path "path"
	"time"
	"github.com/wfxiang08/thrift_rpc_base/zkhelper"
)

type ServiceEndpoint struct {
	Service       string `json:"service"`
	ServiceId     string `json:"service_id"`
	Frontend      string `json:"frontend"`
	DeployPath    string `json:"deploy_path"`
	CodeUrlVerion string `json:"code_url_version"`
	Hostname      string `json:"hostname"`
	StartTime     string `json:"start_time"`
}

func NewServiceEndpoint(service string, serviceId string, frontend string,
	deployPath string, codeUrlVerion string) *ServiceEndpoint {

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "Unknown"
	}

	startTime := FormatYYYYmmDDHHMMSS(time.Now())

	return &ServiceEndpoint{
		Service:       service,
		ServiceId:     serviceId,
		Frontend:      frontend,
		DeployPath:    deployPath,
		CodeUrlVerion: codeUrlVerion,
		Hostname:      hostname,
		StartTime:     startTime,
	}
}

//
// 删除Service Endpoint
//
func (s *ServiceEndpoint) DeleteServiceEndpoint(top *Topology) {
	path := top.ProductServiceEndPointPath(s.Service, s.ServiceId)
	if ok, _ := top.Exist(path); ok {
		zkhelper.DeleteRecursive(top.ZkConn, path, -1)
		log.Println(Red("DeleteServiceEndpoint"), "Path: ", path)
	}
}

//
// 注册一个服务的Endpoints
//
func (s *ServiceEndpoint) AddServiceEndpoint(topo *Topology) error {
	path := topo.ProductServiceEndPointPath(s.Service, s.ServiceId)
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}

	// 创建Service(XXX: Service本身不包含数据)
	CreateRecursive(topo.ZkConn, os_path.Dir(path), "", 0, zkhelper.DefaultDirACLs())

	// 当前的Session挂了，服务就下线
	// topo.FlagEphemeral

	// 参考： https://www.box.com/blog/a-gotcha-when-using-zookeeper-ephemeral-nodes/
	// 如果之前的Session信息还存在，则先删除；然后再添加
	topo.ZkConn.Delete(path, -1)
	var pathCreated string
	pathCreated, err = topo.ZkConn.Create(path, []byte(data), int32(zookeeper.FlagEphemeral), zkhelper.DefaultFileACLs())

	log.Println(Green("AddServiceEndpoint"), "Path: ", pathCreated, ", Error: ", err)
	return err
}

func GetServiceEndpoint(top *Topology, service string, serviceId string) (endpoint *ServiceEndpoint, err error) {

	path := top.ProductServiceEndPointPath(service, serviceId)
	data, _, err := top.ZkConn.Get(path)
	if err != nil {
		return nil, err
	}
	endpoint = &ServiceEndpoint{}
	err = json.Unmarshal(data, endpoint)
	if err != nil {
		return nil, err
	} else {
		return endpoint, nil
	}
}
