//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wfxiang08/cyutils/utils/atomic2"
	"github.com/wfxiang08/cyutils/utils/errors"
	log "github.com/wfxiang08/cyutils/utils/rolling_log"
	"github.com/wfxiang08/go_thrift/thrift"
	"github.com/wfxiang08/thrift_rpc_base/rpc_utils"
)

type BackendConnStateChanged interface {
	StateChanged(conn *BackendConn)
}

var (
	backendConnIndex      int32       = 0
	backendConnIndexMutex *sync.Mutex = &sync.Mutex{}
)

type BackendConn struct {
	addr    string
	service string
	input   chan *Request // 输入的请求, 有: 1024个Buffer

	// seqNum2Request 读写基本上差不多
	seqNumRequestMap *RequestMap
	currentSeqId     int32 // 范围: 1 ~ 100000
	minSeqId         int32
	maxSeqId         int32

	Index    int
	delegate *BackService

	IsMarkOffline atomic2.Bool // 是否标记下线
	IsConnActive  atomic2.Bool // 是否处于Active状态呢
	verbose       bool

	hbLastTime atomic2.Int64
	hbTicker   *time.Ticker
}

func NewBackendConn(addr string, delegate *BackService, service string, verbose bool) *BackendConn {
	requestMap, _ := NewRequestMap(4096)

	var minSeqId int32
	backendConnIndexMutex.Lock()
	log.Infof("Create Backend Conn with index: %d", backendConnIndex)

	// 主要是区分不同的: backendConnIndex
	minSeqId = backendConnIndex

	backendConnIndex += 1
	if backendConnIndex > 100 {
		backendConnIndex = 0
	}
	backendConnIndexMutex.Unlock()

	// BACKEND_CONN_MAX_SEQ_ID = 1000000
	minSeqId = minSeqId * BACKEND_CONN_MAX_SEQ_ID

	bc := &BackendConn{
		addr:             addr,
		service:          service,
		input:            make(chan *Request, 1024),
		seqNumRequestMap: requestMap,

		currentSeqId: minSeqId,
		minSeqId:     minSeqId,
		maxSeqId:     minSeqId + BACKEND_CONN_MAX_SEQ_ID - 1,
		Index:        INVALID_ARRAY_INDEX,

		delegate: delegate,
		verbose:  verbose,
	}
	go bc.Run()
	return bc
}

//
// MarkOffline发生场景:
// 1. 后端服务即将下线，预先通知
// 2. 后端服务已经挂了，zk检测到
//
// BackendConn 在这里暂时理解关闭conn, 而是从 backend_service_proxy中下线当前的conn,
// 然后conn的关闭根据 心跳&Conn的读写异常来判断; 因此 IsConnActive = false 情况下，心跳不能关闭
//
func (bc *BackendConn) MarkOffline() {
	if !bc.IsMarkOffline.Get() {
		log.Printf(Magenta("[%s]BackendConn: %s MarkOffline"), bc.service, bc.addr)
		bc.IsMarkOffline.Set(true)

		// 不再接受(来自backend_service_proxy的)新的输入
		bc.MarkConnActiveFalse()

		close(bc.input)
	}
}

func (bc *BackendConn) MarkConnActiveFalse() {
	if bc.IsConnActive.Get() {
		// 从Active切换到非正常状态
		bc.IsConnActive.Set(false)

		if bc.delegate != nil {
			bc.delegate.StateChanged(bc) // 通知其他人状态出现问题
		}

		// 日志延后, 控制信息尽快生效
		log.Printf(Red("[%s]MarkConnActiveFalse: %s, %p"), bc.service, bc.addr, bc.delegate)
	}
}

//
// 从Active切换到非正常状态
//
func (bc *BackendConn) MarkConnActiveOK() {
	//	if !bc.IsConnActive {
	//		log.Printf(Green("MarkConnActiveOK: %s, %p"), bc.addr, bc.delegate)
	//	}

	bc.IsConnActive.Set(true)
	if bc.delegate != nil {
		bc.delegate.StateChanged(bc) // 通知其他人状态出现问题
	}

}

func (bc *BackendConn) Addr() string {
	return bc.addr
}

//
// 目前有两类请求:
// 1. ping request
// 2. 正常的请求
func (bc *BackendConn) PushBack(r *Request) {
	if bc.IsConnActive.Get() && !bc.IsMarkOffline.Get() {
		// 1. 处于Active状态，并且没有标记下线, 则将 Request 添加到 input 中
		r.Wait.Add(1)
		bc.input <- r
	} else {
		// 2. 直接报错（返回)
		r.Response.Err = errors.New(fmt.Sprintf("[%s] Request Assigned to inactive BackendConn", bc.service))
		log.Warn(Magenta("Push Request To Inactive Backend"))
	}
}

//
// 确保Socket成功连接到后端服务器
//
func (bc *BackendConn) ensureConn() (transport thrift.TTransport, err error) {
	// 1. 创建连接(只要IP没有问题， err一般就是空)
	timeout := time.Second * 5
	if strings.Contains(bc.addr, ":") {
		transport, err = thrift.NewTSocketTimeout(bc.addr, timeout)
	} else {
		transport, err = rpc_utils.NewTUnixDomainTimeout(bc.addr, timeout)
	}
	log.Printf(Cyan("[%s]Create Socket To: %s"), bc.service, bc.addr)

	if err != nil {
		log.ErrorErrorf(err, "[%s]Create Socket Failed: %v, Addr: %s", err, bc.service, bc.addr)
		// 连接不上，失败
		return nil, err
	}

	// 2. 只要服务存在，一般不会出现err
	sleepInterval := 1
	err = transport.Open()
	for err != nil && !bc.IsMarkOffline.Get() {
		log.ErrorErrorf(err, "[%s]Socket Open Failed: %v, Addr: %s", bc.service, err, bc.addr)

		// Sleep: 1, 2, 4这几个间隔
		time.Sleep(time.Duration(sleepInterval) * time.Second)

		if sleepInterval < 4 {
			sleepInterval *= 2
		}
		err = transport.Open()
	}
	return transport, err
}

//
// 不断建立到后端的逻辑，负责: BackendConn#input到redis的数据的输入和返回
//
func (bc *BackendConn) Run() {

	for k := 0; !bc.IsMarkOffline.Get(); k++ {

		// 1. 首先BackendConn将当前 input中的数据写到后端服务中
		transport, err := bc.ensureConn()
		if err != nil {
			log.ErrorErrorf(err, "[%s]BackendConn#ensureConn error: %v", bc.service, err)
			return
		}

		connOver := &sync.WaitGroup{}
		c := NewTBufferedFramedTransport(transport, 100*time.Microsecond, 20)

		bc.MarkConnActiveOK() // 准备接受数据
		connOver.Add(1)
		bc.loopReader(c, connOver) // 异步(读取来自后端服务器的返回数据)
		// 2. 将 bc.input 中的请求写入 后端的Rpc Server
		err = bc.loopWriter(c) // 同步

		// 3. 停止接受Request
		bc.MarkConnActiveFalse()

		// 等待Conn正式关闭
		connOver.Wait()

		// 4. 将bc.input中剩余的 Request直接出错处理
		if err == nil {
			log.Printf(Red("[%s]BackendConn#loopWriter normal Exit..."), bc.service)
			break
		} else {
			// 对于尚未处理的Request, 直接报错
			for i := len(bc.input); i != 0; i-- {
				r := <-bc.input
				bc.setResponse(r, nil, err)
			}
		}
	}
}

//
// 将 bc.input 中的Request写入后端的服务器
//
func (bc *BackendConn) loopWriter(c *TBufferedFramedTransport) error {

	defer func() {
		// 关闭心跳的Ticker
		bc.hbTicker.Stop()
		bc.hbTicker = nil
	}()

	var r *Request
	var ok bool

	// 准备HB Ticker
	bc.hbTicker = time.NewTicker(time.Second)
	bc.hbLastTime.Set(time.Now().Unix())

	for true {
		// 等待输入的Event, 或者 heartbeatTimeout
		select {
		case <-bc.hbTicker.C:
			if time.Now().Unix()-bc.hbLastTime.Get() > HB_TIMEOUT {
				return errors.New(fmt.Sprintf("[%s]HB timeout", bc.service))
			} else {
				// 定时添加Ping的任务; 如果标记下线，则不在心跳
				if !bc.IsMarkOffline.Get() {
					// 发送心跳信息
					r := NewPingRequest()
					bc.PushBack(r)

					// 同时检测当前的异常请求
					expired := microseconds() - REQUEST_EXPIRED_TIME_MICRO // 以microsecond为单位
					// microseconds() - request.Start > REQUEST_EXPIRED_TIME_MICRO
					// 超时: microseconds() - REQUEST_EXPIRED_TIME_MICRO > request.Start
					bc.seqNumRequestMap.RemoveExpired(expired)

				}
			}

		case r, ok = <-bc.input:
			if !ok {
				return nil
			} else {
				//
				// 如果暂时没有数据输入，则p策略可能就有问题了
				// 只有写入数据，才有可能产生flush; 如果是最后一个数据必须自己flush, 否则就可能无限期等待
				//
				if r.Request.TypeId == MESSAGE_TYPE_HEART_BEAT {
					// 过期的HB信号，直接放弃
					if time.Now().Unix()-r.Start > 4 {
						log.Printf(Magenta("Expired HB Signal"))
					}
				}

				// 请求正常转发给后端的Rpc Server
				var flush = len(bc.input) == 0

				// 1. 替换新的SeqId
				r.ReplaceSeqId(bc.currentSeqId)
				bc.IncreaseCurrentSeqId()

				// 2. 主动控制Buffer的flush
				// 先记录SeqId <--> Request, 再发送请求
				// 否则: 请求从后端返回，记录还没有完成，就容易导致Request丢失
				bc.seqNumRequestMap.Add(r.Response.SeqId, r)

				// 2. 主动控制Buffer的flush
				c.Write(r.Request.Data)
				err := c.FlushBuffer(flush)

				if err == nil {
					log.Debugf("--> SeqId: %d vs. %d To Backend", r.Request.SeqId, r.Response.SeqId)

				} else {
					bc.seqNumRequestMap.Pop(r.Response.SeqId) // 如果写错了，在删除
					// 进入不可用状态(不可用状态下，通过自我心跳进入可用状态)
					return bc.setResponse(r, nil, err)
				}
			}
		}
	}

	return nil
}

//
// Client <---> Proxy[BackendConn] <---> RPC Server[包含LB]
// BackConn <====> RPC Server
// loopReader从RPC Server读取数据，然后根据返回的结果来设置: Client的Request的状态
//
// 1. bc.flushRequest
// 2. bc.setResponse
//
func (bc *BackendConn) loopReader(c *TBufferedFramedTransport, connOver *sync.WaitGroup) {
	go func() {
		defer connOver.Done()
		defer c.Close()

		lastTime := time.Now().Unix()
		// Active状态，或者最近5s有数据返回
		// 设计理由：服务在线，则请求正常发送；
		//         服务下线后，则期待后端服务的数据继续返回(最多等待5s)
		for bc.IsConnActive.Get() || (time.Now().Unix()-lastTime < 5) {
			// 读取来自后端服务的数据，通过 setResponse 转交给 前端
			// client <---> proxy <-----> backend_conn <---> rpc_server
			// ReadFrame需要有一个度? 如果碰到EOF该如何处理呢?

			// io.EOF在两种情况下会出现
			//
			resp, err := c.ReadFrame()
			lastTime = time.Now().Unix()
			if err != nil {
				err1, ok := err.(thrift.TTransportException)
				if !ok || err1.TypeId() != thrift.END_OF_FILE {
					log.ErrorErrorf(err, Red("[%s]ReadFrame From Server with Error: %v"), bc.service, err)
				}
				bc.flushRequests(err)
				break
			} else {

				bc.setResponse(nil, resp, err)
			}
		}

		bc.flushRequests(errors.New("BackendConn Timeout"))
	}()
}

// 处理所有的等待中的请求
func (bc *BackendConn) flushRequests(err error) {
	// 告诉BackendService, 不再接受新的请求
	bc.MarkConnActiveFalse()

	seqRequest := bc.seqNumRequestMap.Purge()
	for _, request := range seqRequest {
		request.Response.Err = err
		request.Wait.Done()
		log.Debugf("FlushRequests, SeqId: %d", request.Response.SeqId)
	}

}

// 配对 Request, resp, err
// PARAM: resp []byte 为一帧完整的thrift数据包
func (bc *BackendConn) setResponse(r *Request, data []byte, err error) error {
	// 表示出现错误了
	if data == nil {
		log.Debugf("[%s] SeqId: %d, No Data From Server, error: %v", r.Service, r.Response.SeqId, err)
		r.Response.Err = err
	} else {
		// 从resp中读取基本的信息
		typeId, method, seqId, err := DecodeThriftTypIdSeqId(data)
		//		if err != nil {
		//			log.Debugf("SeqId: %d, Decoded, error: %v", seqId, err)
		//		} else {
		//			log.Debugf("SeqId: %d, Decoded", seqId)
		//		}
		// 解码错误，直接报错
		if err != nil {
			log.Debugf("SeqId: %d, Decoded, error: %v", seqId, err)
			return err
		}

		// 找到对应的Request
		req := bc.seqNumRequestMap.Pop(seqId)

		// 如果是心跳，则OK
		if typeId == MESSAGE_TYPE_HEART_BEAT {
			bc.hbLastTime.Set(time.Now().Unix())
			//			if req != nil {
			//				log.Printf("HB RT: %.3fms", float64(microseconds()-req.Start)*0.001)
			//			}
			return nil
		}

		if req == nil {
			// return errors.New("Invalid Response")
			// 由于是异步返回，因此回来找不到也正常
			log.Errorf("#setResponse not found, seqId: %d", seqId)
			return nil
		} else {

			if req.Response.SeqId != seqId {
				log.Errorf("Data From Server, SeqId not match, Ex: %d, Ret: %d", req.Request.SeqId, seqId)
			}
			r = req
			r.Response.TypeId = typeId
			if req.Request.Name != method {
				data = nil
				err = req.NewInvalidResponseError(method, "conn_proxy")
			}
		}
	}

	// 正常返回数据，或者报错
	r.Response.Data, r.Response.Err = data, err

	// 还原SeqId
	if data != nil {
		r.RestoreSeqId()
	}

	// 设置几个控制用的channel
	r.Wait.Done()

	return err
}

func (bc *BackendConn) IncreaseCurrentSeqId() {
	// 备案(只有loopWriter操作，不加锁)
	bc.currentSeqId++
	if bc.currentSeqId > bc.maxSeqId {
		bc.currentSeqId = bc.minSeqId
	}
}
