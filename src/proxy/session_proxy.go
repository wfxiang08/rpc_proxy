//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	"github.com/wfxiang08/go_thrift/thrift"
	log "github.com/wfxiang08/cyutils/utils/rolling_log"
	"time"
)

type Session struct {
	*TBufferedFramedTransport

	RemoteAddress string
	Ops           int64
	LastOpUnix    int64
	CreateUnix    int64
	verbose       bool
}

// c： client <---> proxy之间的连接
func NewSession(c thrift.TTransport, address string, verbose bool) *Session {
	return NewSessionSize(c, address, verbose, 1024 * 32, 5000)
}

func NewSessionSize(c thrift.TTransport, address string, verbose bool,
bufsize int, timeout int) *Session {

	s := &Session{
		CreateUnix:               time.Now().Unix(),
		RemoteAddress:            address,
		verbose:                  verbose,
		TBufferedFramedTransport: NewTBufferedFramedTransport(c, time.Microsecond * 100, 20),
	}

	// Reader 处理Client发送过来的消息
	// Writer 将后端服务的数据返回给Client
	log.Infof(Green("NewSession To: %s"), s.RemoteAddress)
	return s
}

func (s *Session) Close() error {
	log.Printf(Red("Close Proxy Session"))
	return s.TBufferedFramedTransport.Close()
}

// Session是同步处理请求，因此没有必要搞多个
func (s *Session) Serve(d *Router, maxPipeline int) {
	defer func() {
		s.Close()
		log.Infof(Red("==> Session Over: %s, Total %d Ops"), s.RemoteAddress, s.Ops)
		if err := recover(); err != nil {
			log.Infof(Red("Catch session error: %v"), err)
		}
	}()

	requests := make(chan *Request, 20)

	go func() {
		var err error
		for r := range requests {
			// 3. 等待请求处理完毕
			//    先调用的先返回
			s.handleResponse(r)

			// 4. 将结果写回给Client
			if s.verbose {
				log.Debugf("[%s]Session#loopWriter --> client FrameSize: %d",
					r.Service, len(r.Response.Data))
			}

			// 5. 将请求返回给Client, r.Response.Data ---> Client
			_, err = s.TBufferedFramedTransport.Write(r.Response.Data)
			r.Recycle() // 重置: Request

			if err != nil {
				// 写数据出错，那只能说明连接坏了，直接断开
				log.ErrorErrorf(err, "Write back Data Error: %v", err)
				return
			}

			// 6. Flush
			err = s.TBufferedFramedTransport.FlushBuffer(true) // len(tasks) == 0
			if err != nil {
				log.ErrorErrorf(err, "Write back Data Error: %v", err)
				return
			}
		}

	}()

	// 读写分离
	for true {
		// 1. 读取请求
		request, err := s.ReadFrame()
		s.Ops += 1

		// 读取出错，直接退出
		if err != nil {
			err1, ok := err.(thrift.TTransportException)
			if !ok || err1.TypeId() != thrift.END_OF_FILE {
				log.ErrorErrorf(err, Red("ReadFrame Error: %v"), err)
			}
			// r.Recycle()
			close(requests)
			return
		}

		var r *Request
		// 2. 处理请求
		r, err = s.handleRequest(request, d)
		if err != nil {
			// r.Recycle() // 出错之后也要主动返回数据
			log.ErrorErrorf(err, Red("handleRequest Error: %v"), err)
			r.Response.Err = err
		}

		requests <- r
	}
}

//
//
// 等待Request请求的返回: Session最终被Block住
//
func (s *Session) handleResponse(r *Request) {
	// 等待结果的出现
	r.Wait.Wait()

	// 将Err转换成为Exception
	if r.Response.Err != nil {
		r.Response.Data = GetThriftException(r, "proxy_session")
		log.Infof(Magenta("---->Convert Error Back to Exception, Err: %v"), r.Response.Err)
	}

	// 如何处理Data和Err呢?
	incrOpStats(r.Request.Name, microseconds() - r.Start)
}

// 处理来自Client的请求
func (s *Session) handleRequest(request []byte, d *Router) (*Request, error) {
	// 构建Request
	if s.verbose {
		log.Printf("HandleRequest: %s", string(request))
	}
	r, err := NewRequest(request, true)
	if err != nil {
		return r, err
	}

	// 增加统计
	s.LastOpUnix = time.Now().Unix()
	s.Ops++
	if r.Request.TypeId == MESSAGE_TYPE_HEART_BEAT {
		HandleProxyPingRequest(r) // 直接返回数据
		return r, nil
	}

	// 交给Dispatch
	// Router
	return r, d.Dispatch(r)
}

func microseconds() int64 {
	return time.Now().UnixNano() / int64(time.Microsecond)
}
