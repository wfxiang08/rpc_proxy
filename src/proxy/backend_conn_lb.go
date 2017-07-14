//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	"fmt"
	"time"

	"github.com/wfxiang08/cyutils/utils/atomic2"
	"github.com/wfxiang08/cyutils/utils/errors"
	log "github.com/wfxiang08/cyutils/utils/rolling_log"
	"github.com/wfxiang08/go_thrift/thrift"
)

type BackendConnLBStateChanged interface {
	StateChanged(conn *BackendConnLB)
}

type BackendConnLB struct {
	transport   thrift.TTransport
	address     string
	serviceName string
	input       chan *Request // 输入的请求, 有: 1024个Buffer

	seqNumRequestMap *RequestMap
	currentSeqId     int32 // 范围: 1 ~ 100000
	Index            int
	delegate         BackendConnLBStateChanged
	verbose          bool
	IsConnActive     atomic2.Bool // 是否处于Active状态呢

	hbLastTime atomic2.Int64
	hbTicker   *time.Ticker
}

//
// LB(Load Balancer)启动一个Server和后端的服务(Backend)通信， 后端服务负责主动连接LB；
// 1. LB负责定期地和Backend进行ping/pang;
//    如果LB发现Backend长时间没有反应，或者出错，则端口和Backend之间的连接
// 2. Backend根据config.ini主动注册LB, 按照一定的策略重连
//
// BackendConnLB
//   1. 为Backend主动向LB注册之后，和LB之间建立的一条Connection
//   2. 底层的conn在LB BackendService中accepts时就已经建立好，因此BackendConnLB
//      就是建立在transport之上的控制逻辑
//
func NewBackendConnLB(transport thrift.TTransport, serviceName string,
	address string, delegate BackendConnLBStateChanged,
	verbose bool) *BackendConnLB {
	requestMap, _ := NewRequestMap(4096)
	bc := &BackendConnLB{
		transport:   transport,
		address:     address,
		serviceName: serviceName,
		input:       make(chan *Request, 1024),

		seqNumRequestMap: requestMap,
		currentSeqId:     BACKEND_CONN_MIN_SEQ_ID,

		Index: INVALID_ARRAY_INDEX, // 用于记录: BackendConnLB在数组中的位置

		delegate: delegate,
		verbose:  verbose,
	}
	bc.IsConnActive.Set(true)
	go bc.Run()
	return bc
}

func (bc *BackendConnLB) MarkConnActiveFalse() {
	// 从Active切换到非正常状态
	if bc.IsConnActive.CompareAndSwap(true, false) && bc.delegate != nil {
		// bc.IsConnActive.Set(false)
		bc.delegate.StateChanged(bc) // 通知其他人状态出现问题
	} else {
		bc.IsConnActive.Set(false)
	}
}

// run之间 transport刚刚建立，因此服务的可靠性比较高
func (bc *BackendConnLB) Run() {
	log.Printf(Green("[%s]Add New BackendConnLB: %s"), bc.serviceName, bc.address)

	// 1. 首先BackendConn将当前 input中的数据写到后端服务中
	err := bc.loopWriter()

	// 2. 从Active切换到非正常状态, 同时不再从backend_service_lb接受新的任务
	//    可能出现异常，也可能正常退出(反正不干活了)
	bc.MarkConnActiveFalse()

	log.Printf(Red("[%s]Remove Faild BackendConnLB: %s"), bc.serviceName, bc.address)

	if err == nil {
		// bc.input被关闭了，应该就没有 Request 了
	} else {
		// 如果出现err, 则将bc.input中现有的数据都flush回去（直接报错)
		for i := len(bc.input); i != 0; i-- {
			r := <-bc.input
			bc.setResponse(r, nil, err)
		}
	}

}

func (bc *BackendConnLB) Address() string {
	return bc.address
}

//
// Request为将要发送到后端进程请求，包括lb层的心跳，或来自前端的正常请求
//
func (bc *BackendConnLB) PushBack(r *Request) {
	// 关键路径必须有Log, 高频路径的Log需要受verbose状态的控制
	if bc.IsConnActive.Get() {
		// log.Printf("Push Request to backend: %s", r.Request.Name)
		r.Service = bc.serviceName
		r.Wait.Add(1)
		bc.input <- r

	} else {
		r.Response.Err = errors.New(fmt.Sprintf("[%s] Request Assigned to inactive BackendConnLB", bc.serviceName))
		log.Warn(Magenta("Push Request To Inactive Backend"))
	}

}

//
// 数据: LB ---> backend services
//
// 如果input关闭，且loopWriter正常处理完毕之后，返回nil
// 其他情况返回error
//
func (bc *BackendConnLB) loopWriter() error {
	// 正常情况下, ok总是为True; 除非bc.input的发送者主动关闭了channel, 表示再也没有新的Task过来了
	// 参考: https://tour.golang.org/concurrency/4
	// 如果input没有关闭，则会block
	c := NewTBufferedFramedTransport(bc.transport, 100*time.Microsecond, 20)

	// bc.MarkConnActiveOK() // 准备接受数据
	// BackendConnLB 在构造之初就有打开的transport, 并且Active默认为OK

	defer func() {
		bc.hbTicker.Stop()
		bc.hbTicker = nil
	}()

	// 从"RPC Backend" RPC Worker 中读取结果, 然后返回给proxy
	bc.loopReader(c) // 异步

	// 建立连接之后，就启动HB
	bc.hbTicker = time.NewTicker(time.Second)
	bc.hbLastTime.Set(time.Now().Unix())

	var r *Request
	var ok bool

	for true {
		// 等待输入的Event, 或者 heartbeatTimeout
		select {
		case <-bc.hbTicker.C:
			// 两种情况下，心跳会超时
			// 1. 对方挂了
			// 2. 自己快要挂了，然后就不再发送心跳；没有了心跳，就会超时
			if time.Now().Unix()-bc.hbLastTime.Get() > HB_TIMEOUT {
				// 强制关闭c
				c.Close()
				return errors.New("Worker HB timeout")
			} else {
				if bc.IsConnActive.Get() {
					// 定时添加Ping的任务
					r := NewPingRequest()
					bc.PushBack(r)

					// 同时检测当前的异常请求
					expired := microseconds() - REQUEST_EXPIRED_TIME_MICRO // 以microsecond为单位
					bc.seqNumRequestMap.RemoveExpired(expired)
				}
			}

		case r, ok = <-bc.input:
			if !ok {
				return nil
			} else {
				// 如果暂时没有数据输入，则p策略可能就有问题了
				// 只有写入数据，才有可能产生flush; 如果是最后一个数据必须自己flush, 否则就可能无限期等待
				//
				if r.Request.TypeId == MESSAGE_TYPE_HEART_BEAT {
					// 过期的HB信号，直接放弃
					if time.Now().Unix()-r.Start > 4 {
						log.Warnf(Red("Expired HB Signals"))
					}
				} else if r.Request.TypeId == MESSAGE_TYPE_STOP_CONFIRM {
					// 强制写一个新的Response到Worker，不关心是否写成功；也不关心反馈结果
					c.Write(r.Request.Data)
					err := c.FlushBuffer(true)

					if err != nil {
						log.ErrorErrorf(err, "Stop confirm Error")
					}
				} else {
					// 先不做优化
					var flush = true // len(bc.input) == 0

					// 1. 替换新的SeqId(currentSeqId只在当前线程中使用, 不需要同步)
					r.ReplaceSeqId(bc.currentSeqId)
					bc.IncreaseCurrentSeqId()

					// 2. 主动控制Buffer的flush
					// 先记录SeqId <--> Request, 再发送请求
					// 否则: 请求从后端返回，记录还没有完成，就容易导致Request丢失
					bc.seqNumRequestMap.Add(r.Response.SeqId, r)
					c.Write(r.Request.Data)
					err := c.FlushBuffer(flush)

					if err != nil {
						bc.seqNumRequestMap.Pop(r.Response.SeqId)
						log.ErrorErrorf(err, "FlushBuffer Error: %v\n", err)

						// 进入不可用状态(不可用状态下，通过自我心跳进入可用状态)
						return bc.setResponse(r, nil, err)
					}
				}
			}
		}

	}
	return nil
}

//
// 从"RPC Backend" RPC Worker 中读取结果, ReadFrame读取的是一个thrift message
// 存在两种情况:
// 1. 正常读取thrift message, 然后从frame解码得到seqId, 然后得到request, 结束请求
// 2. 读取错误
//    将现有的requests全部flush回去
//
func (bc *BackendConnLB) loopReader(c *TBufferedFramedTransport) {
	go func() {
		defer c.Close()

		for true {
			// 坚信: EOF只有在连接被关闭的情况下才会发生，其他情况下, Read等操作被会被block住
			// EOF有两种情况:
			// 1. 连接正常关闭，最后数据等完整读取 --> io.EOF
			// 2. 连接异常关闭，数据不完整 --> io.ErrUnexpectedEOF
			//
			// rpc_server ---> backend_conn
			frame, err := c.ReadFrame() // 有可能被堵住

			if err != nil {
				// 如果出错，则Flush所有的请求
				err1, ok := err.(thrift.TTransportException)
				if !ok || err1.TypeId() != thrift.END_OF_FILE {
					log.ErrorErrorf(err, Red("ReadFrame From rpc_server with Error: %v\n"), err)
				}
				// TODO: 可能需要细化，有些错误出现之后，可能需要给其他的请求一些机会
				bc.flushRequests(err)
				break
			} else {
				bc.setResponse(nil, frame, err)
			}
		}
	}()
}

// 处理所有的等待中的请求
func (bc *BackendConnLB) flushRequests(err error) {
	// 告诉BackendService, 不再接受新的请求
	bc.MarkConnActiveFalse()

	seqRequest := bc.seqNumRequestMap.Purge()

	for _, request := range seqRequest {
		if request.Request.TypeId == MESSAGE_TYPE_HEART_BEAT {
			// 心跳出错了，则直接直接跳过
		} else {
			log.Printf(Red("Handle Failed Request: %s.%s"), request.Service, request.Request.Name)
			request.Response.Err = err
			request.Wait.Done()
		}
	}

	// 关闭输入
	close(bc.input)

}

// 配对 Request, resp, err
// PARAM: resp []byte 为一帧完整的thrift数据包
func (bc *BackendConnLB) setResponse(r *Request, data []byte, err error) error {
	//	log.Printf("#setResponse:  data: %v", data)
	// 表示出现错误了
	if data == nil {
		log.Printf("No Data From Server, error: %v\n", err)
		r.Response.Err = err
	} else {
		// 从resp中读取基本的信息
		typeId, method, seqId, err := DecodeThriftTypIdSeqId(data)

		// 解码错误，直接报错
		if err != nil {
			log.ErrorErrorf(err, "Decode SeqId Error: %v", err)
			return err
		}

		if typeId == MESSAGE_TYPE_STOP {
			// 不再接受新的输入
			// 直接来自后端的服务(不遵循: Request/Reply模型)
			// 或者回传给后端一个确认停止消息
			bc.MarkConnActiveFalse()

			// 临时再发送一个请求; 有些语言没有异步，不太方便设置timeout, 那么通过stop confirm告知这是最后一个请求
			r := NewStopConfirmRequest()
			r.Service = bc.serviceName
			r.Wait.Add(1)
			bc.input <- r

			return nil
		}

		// 找到对应的Request

		req := bc.seqNumRequestMap.Pop(seqId)

		// 如果是心跳，则OK
		if typeId == MESSAGE_TYPE_HEART_BEAT {
			bc.hbLastTime.Set(time.Now().Unix())
			return nil
		}

		if req == nil {
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
				err = req.NewInvalidResponseError(method, "conn_lb")
			}
		}
	}

	r.Response.Data, r.Response.Err = data, err
	// 还原SeqId
	if data != nil {
		r.RestoreSeqId()
	}


	r.Wait.Done()
	return err
}

func (bc *BackendConnLB) IncreaseCurrentSeqId() {
	// 备案(只有loopWriter操作，不加锁)
	bc.currentSeqId++
	if bc.currentSeqId > BACKEND_CONN_MAX_SEQ_ID {
		bc.currentSeqId = BACKEND_CONN_MIN_SEQ_ID
	}
}
