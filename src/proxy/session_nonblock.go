//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	"github.com/wfxiang08/cyutils/utils/atomic2"
	"github.com/wfxiang08/cyutils/utils/errors"
	log "github.com/wfxiang08/cyutils/utils/rolling_log"
	"github.com/wfxiang08/go_thrift/thrift"
	"sync"
	"time"
)

//
// 用于rpc proxy或者load balance用来管理Client的
//
type NonBlockSession struct {
	*TBufferedFramedTransport

	RemoteAddress   string

	closed          atomic2.Bool
	verbose         bool

	// 用于记录整个RPC服务的最后的访问时间，然后用于Graceful Stop
	lastRequestTime *atomic2.Int64

	lastSeqId       int32
}

func NewNonBlockSession(c thrift.TTransport, address string, verbose bool,
lastRequestTime *atomic2.Int64) *NonBlockSession {
	return NewNonBlockSessionSize(c, address, verbose, lastRequestTime, 1024 * 32, 1800)
}

func NewNonBlockSessionSize(c thrift.TTransport, address string, verbose bool,
lastRequestTime *atomic2.Int64, bufsize int, timeout int) *NonBlockSession {
	s := &NonBlockSession{
		RemoteAddress:            address,
		lastRequestTime:          lastRequestTime,
		verbose:                  verbose,
		TBufferedFramedTransport: NewTBufferedFramedTransport(c, time.Microsecond * 100, 20),
	}

	// 还是基于c net.Conn进行读写，只是采用Redis协议进行编码解码
	// Reader 处理Client发送过来的消息
	// Writer 将后端服务的数据返回给Client
	log.Printf(Green("Session From Proxy [%s] created"), address)
	return s
}

func (s *NonBlockSession) Close() error {
	s.closed.Set(true)
	return s.TBufferedFramedTransport.Close()
}

func (s *NonBlockSession) IsClosed() bool {
	return s.closed.Get()
}

func (s *NonBlockSession) Serve(d Dispatcher, maxPipeline int) {
	var errlist errors.ErrorList

	defer func() {
		// 只限制第一个Error
		if err := errlist.First(); err != nil {
			log.Infof("Session [%p] closed, Error = %v", s, err)
		} else {
			log.Infof("Session [%p] closed, Quit", s)
		}
	}()

	// 来自connection的各种请求
	tasks := make(chan *Request, maxPipeline)
	go func() {
		defer func() {
			// 出现错误了，直接关闭Session
			s.Close()

			log.Infof(Red("Session [%p] closed, Abandon %d Tasks"), s, len(tasks))

			for _ = range tasks {
				// close(tasks)关闭for loop

			}
		}()
		if err := s.loopWriter(tasks); err != nil {
			errlist.PushBack(err)
		}
	}()

	// 用于等待for中的go func执行完毕
	var wait sync.WaitGroup
	for true {
		// Reader不停地解码， 将Request
		request, err := s.ReadFrame()

		if err != nil {
			errlist.PushBack(err)
			break
		}
		// 来自proxy的请求, request中不带有service
		r, err1 := NewRequest(request, false)
		if err1 != nil {
			errlist.PushBack(err1)
			break
		}

		wait.Add(1)
		go func(r*Request) {
			// 异步执行(直接通过goroutine来调度，因为: SessionNonBlock中的不同的Request相互独立)

			_ = s.handleRequest(r, d)
			//			if r.Request.TypeId != MESSAGE_TYPE_HEART_BEAT {
			//				log.Debugf(Magenta("[%p] --> SeqId: %d, Goroutine: %d"), s, r.Request.SeqId, runtime.NumGoroutine())
			//			}

			// 数据请求完毕之后，将Request交给tasks, 然后再写回Client

			tasks <- r
			wait.Done()
			//			if r.Request.TypeId != MESSAGE_TYPE_HEART_BEAT {
			//				log.Printf("Time RT: %.3fms", float64(microseconds()-r.Start)*0.001)
			//			}

		}(r)
	}
	// 等待go func执行完毕
	wait.Wait()
	close(tasks)
	return
}

//
//
// NonBlock和Block的区别:
// NonBlock的 Request和Response是不需要配对的， Request和Response是独立的，例如:
//  ---> RequestA, RequestB
//  <--- RequestB, RequestA 后请求的，可以先返回
//
func (s *NonBlockSession) loopWriter(tasks <-chan *Request) error {
	for r := range tasks {
		// 1. tasks中的请求是已经请求完毕的，loopWriter负责将它们的数据写回到rpc proxy
		s.handleResponse(r)

		// 2. 将结果写回给Client
		_, err := s.TBufferedFramedTransport.Write(r.Response.Data)
		r.Recycle()
		if err != nil {
			log.ErrorErrorf(err, "SeqId: %d, Write back Data Error: %v\n", r.Request.SeqId, err)
			return err
		}

		//		typeId, sedId, _ := DecodeThriftTypIdSeqId(r.Response.Data)

		//		if typeId != MESSAGE_TYPE_HEART_BEAT {
		//			if sedId != s.lastSeqId {
		//				log.Errorf(Red("Invalid SedId for Writer: %d vs. %d"), sedId, s.lastSeqId)
		//			}
		//		}

		// log.Printf(Magenta("Task: %d"), r.Response.SeqId)

		// 3. Flush
		err = s.TBufferedFramedTransport.FlushBuffer(len(tasks) == 0) // len(tasks) == 0
		//		if err != nil {
		//			log.Debugf(Magenta("Write Back to Client/Proxy SeqId: %d, Error: %v"), r.Request.SeqId, err)
		//		} else {
		//			log.Debugf(Magenta("Write Back to Client/Proxy SeqId: %d"), r.Request.SeqId)
		//		}

		if err != nil {
			return err
		}
	}
	return nil
}

// 获取可以直接返回给Client的response
func (s *NonBlockSession) handleResponse(r *Request) {

	// 将Err转换成为Exception
	if r.Response.Err != nil {
		log.Println("#handleResponse, Error ----> Reponse Data")
		r.Response.Data = GetThriftException(r, "nonblock_session")
	}

	incrOpStats(r.Request.Name, microseconds() - r.Start)
}

// 处理来自Client的请求
// 将它的请教交给后端的Dispatcher
//
func (s *NonBlockSession) handleRequest(r *Request, d Dispatcher) error {
	// 构建Request
	//	log.Printf("HandleRequest: %s\n", string(request))
	// 处理心跳
	if r.Request.TypeId == MESSAGE_TYPE_HEART_BEAT {
		//		log.Printf(Magenta("PING/PANG"))
		HandlePingRequest(r)
		return nil
	}

	//	if r.Request.SeqId-s.lastSeqId != 1 {
	//		log.Errorf(Red("Invalid SedId: %d vs. %d"), r.Request.SeqId,
	//			s.lastSeqId)
	//	}
	//	s.lastSeqId = r.Request.SeqId

	// 正常请求
	if s.lastRequestTime != nil {
		s.lastRequestTime.Set(time.Now().Unix())
	}

	//	log.Debugf("Before Dispatch, SeqId: %d", r.Request.SeqId)

	// 交给Dispatch
	return d.Dispatch(r)
}
