//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	"encoding/json"
	"fmt"
	"github.com/wfxiang08/cyutils/utils/errors"
	log "github.com/wfxiang08/cyutils/utils/rolling_log"
	topo "github.com/wfxiang08/go-zookeeper/zk"
	"github.com/wfxiang08/thrift_rpc_base/zkhelper"
	color "github.com/fatih/color"
	os_path "path"
	"strings"
)

var green = color.New(color.FgGreen).SprintFunc()

//
// 设计方案:
// 1. zk中保存如下的数据
//    /chunyu/ProductName/Services/Services1
//        								  /host1:port2
//        								  /host2:port2
//                                /Services2
//
//
// Topology需要关注:
// /chunyu/ProductName/Services 下面的Children的变化，关注有哪些服务，能自动发现
// 关注每一个服务的变化
// 这个和Codis的区别和联系?
//
type Topology struct {
	ProductName string        // 例如: 线上服务， 测试服务等等
	zkAddr      string        // zk的地址
	ZkConn      zkhelper.Conn // zk的连接
	basePath    string
}

// 春雨产品服务列表对应的Path
func (top *Topology) productBasePath(productName string) string {
	return fmt.Sprintf("/zk/product/%s", productName)
}
func (top *Topology) ProductServicesPath() string {
	return fmt.Sprintf("%s/services", top.basePath)
}

func (top *Topology) ProductServicePath(service string) string {
	return fmt.Sprintf("%s/services/%s", top.basePath, service)
}

// 获取具体的某个EndPoint的Path
func (top *Topology) ProductServiceEndPointPath(service string, endpoint string) string {
	return fmt.Sprintf("%s/services/%s/%s", top.basePath, service, endpoint)
}

func (top *Topology) FullPath(path string) string {
	if !strings.HasPrefix(path, top.basePath) {
		path = fmt.Sprintf("%s%s", top.basePath, path)
	}
	return path
}

// 指定的path是否在zk中存在
func (top *Topology) Exist(path string) (bool, error) {
	path = top.FullPath(path)
	return zkhelper.NodeExists(top.ZkConn, path)
}

func NewTopology(ProductName string, zkAddr string) *Topology {
	// 创建Topology对象，并且初始化ZkConn
	t := &Topology{zkAddr: zkAddr, ProductName: ProductName}
	t.basePath = t.productBasePath(ProductName)
	t.InitZkConn()
	return t
}

func (top *Topology) InitZkConn() {
	var err error
	// 连接到zk
	// 30s的timeout
	top.ZkConn, err = zkhelper.ConnectToZk(top.zkAddr, 30) // 参考: Codis的默认配置
	if err != nil {
		log.PanicErrorf(err, "init failed")
	}
}

func (top *Topology) IsChildrenChangedEvent(e interface{}) bool {
	return e.(topo.Event).Type == topo.EventNodeChildrenChanged
}

func (top *Topology) DeleteDir(path string) {
	dir := top.FullPath(path)
	if ok, _ := top.Exist(dir); ok {
		zkhelper.DeleteRecursive(top.ZkConn, dir, -1)
	}
}

// 创建指定的Path
func (top *Topology) CreateDir(path string) (string, error) {
	dir := top.FullPath(path)
	if ok, _ := top.Exist(dir); ok {
		log.Println("Path Exists")
		return dir, nil
	} else {
		return zkhelper.CreateRecursive(top.ZkConn, dir, "", 0, zkhelper.DefaultDirACLs())
	}
}

func (top *Topology) SetPathData(path string, data []byte) {
	dir := top.FullPath(path)
	top.ZkConn.Set(dir, data, -1)
}

//
// 设置RPC Proxy的数据:
//     绑定的前端的ip/port, 例如: {"rpc_front": "tcp://127.0.0.1:5550"}
//
func (top *Topology) SetRpcProxyData(proxyInfo map[string]interface{}) error {
	path := top.FullPath("/rpc_proxy")
	data, err := json.Marshal(proxyInfo)
	if err != nil {
		return err
	}

	// topo.FlagEphemeral 这里的ProxyInfo是手动配置的，需要持久化
	path, err = CreateOrUpdate(top.ZkConn, path, string(data), 0, zkhelper.DefaultDirACLs(), true)
	log.Println(green("SetRpcProxyData"), "Path: ", path, ", Error: ", err, ", Data: ", string(data))
	return err
}

//
// 读取RPC Proxy的数据:
//     绑定的前端的ip/port, 例如: {"rpc_front": "tcp://127.0.0.1:5550"}
//
func (top *Topology) GetRpcProxyData() (proxyInfo map[string]interface{}, e error) {
	path := top.FullPath("/rpc_proxy")
	data, _, err := top.ZkConn.Get(path)

	log.Println("Data: ", data, ", err: ", err)
	if err != nil {
		return nil, err
	}
	proxyInfo = make(map[string]interface{})
	err = json.Unmarshal(data, &proxyInfo)
	if err != nil {
		return nil, err
	} else {
		return proxyInfo, nil
	}
}

//
// evtch:
// 1. 来自Zookeeper驱动通知
// 2. 该通知最终需要通过 evtbus 传递给其他人
//
func (top *Topology) doWatch(evtch <-chan topo.Event, evtbus chan interface{}) {
	e := <-evtch

	// http://wiki.apache.org/hadoop/ZooKeeper/FAQ
	// 如何处理? 照理说不会发生的
	if e.State == topo.StateExpired || e.Type == topo.EventNotWatching {
		log.Warnf("session expired: %+v", e)
		evtbus <- e
		return
	}

	log.Warnf("topo event %+v", e)

	switch e.Type {
	//case topo.EventNodeCreated:
	//case topo.EventNodeDataChanged:
	case topo.EventNodeChildrenChanged: //only care children changed
		//todo:get changed node and decode event
	default:

		// log.Warnf("%+v", e)
	}

	evtbus <- e
}

func (top *Topology) WatchChildren(path string, evtbus chan interface{}) ([]string, error) {
	// 获取Children的信息
	content, _, evtch, err := top.ZkConn.ChildrenW(path)
	if err != nil {
		return nil, errors.Trace(err)
	}

	go top.doWatch(evtch, evtbus)
	return content, nil
}

// 读取当前path对应的数据，监听之后的事件
func (top *Topology) WatchNode(path string, evtbus chan interface{}) ([]byte, error) {
	content, _, evtch, err := top.ZkConn.GetW(path)
	if err != nil {
		return nil, errors.Trace(err)
	}

	// 从: evtch 读取数据，然后再传递到 evtbus
	// evtbus 是外部可控的 channel
	if evtbus != nil {
		go top.doWatch(evtch, evtbus)
	}
	return content, nil
}

// Create a path and any pieces required, think mkdir -p.
// Intermediate znodes are always created empty.
func CreateRecursive(zconn zkhelper.Conn, zkPath, value string, flags int, aclv []topo.ACL) (pathCreated string, err error) {
	parts := strings.Split(zkPath, "/")
	if parts[1] != zkhelper.MagicPrefix {
		return "", fmt.Errorf("zkutil: non /%v path: %v", zkhelper.MagicPrefix, zkPath)
	}

	pathCreated, err = zconn.Create(zkPath, []byte(value), int32(flags), aclv)

	if zkhelper.ZkErrorEqual(err, topo.ErrNoNode) {
		// Make sure that nodes are either "file" or "directory" to mirror file system
		// semantics.
		dirAclv := make([]topo.ACL, len(aclv))
		for i, acl := range aclv {
			dirAclv[i] = acl
			dirAclv[i].Perms = zkhelper.PERM_DIRECTORY
		}
		_, err = CreateRecursive(zconn, os_path.Dir(zkPath), "", 0, dirAclv)
		if err != nil && !zkhelper.ZkErrorEqual(err, topo.ErrNodeExists) {
			return "", err
		}
		pathCreated, err = zconn.Create(zkPath, []byte(value), int32(flags), aclv)
	}
	return
}

func CreateOrUpdate(zconn zkhelper.Conn, zkPath, value string, flags int, aclv []topo.ACL, recursive bool) (pathCreated string, err error) {
	if recursive {
		pathCreated, err = CreateRecursive(zconn, zkPath, value, flags, aclv)
	} else {
		pathCreated, err = zconn.Create(zkPath, []byte(value), int32(flags), aclv)
	}
	if err != nil && zkhelper.ZkErrorEqual(err, topo.ErrNodeExists) {
		pathCreated = ""
		_, err = zconn.Set(zkPath, []byte(value), -1)
	}
	return
}
