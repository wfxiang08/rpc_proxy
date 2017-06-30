//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	log "github.com/wfxiang08/cyutils/utils/rolling_log"
	"github.com/wfxiang08/go_thrift/thrift"
	"testing"
	"time"
)

//
// go test proxy -v -run "TestBackend"
//
func TestBackend(t *testing.T) {

	// 作为一个Server
	transport, err := thrift.NewTServerSocket("127.0.0.1:0")
	assert.NoError(t, err)
	err = transport.Open() // 打开Transport
	assert.NoError(t, err)

	defer transport.Close()

	err = transport.Listen() // 开始监听
	assert.NoError(t, err)

	addr := transport.Addr().String()

	fmt.Println("Addr: ", addr)

	var requestNum int32 = 10

	requests := make([]*Request, 0, requestNum)

	var i int32
	for i = 0; i < requestNum; i++ {
		buf := make([]byte, 100, 100)
		l := fakeData("Hello", thrift.CALL, i+1, buf[0:0])
		buf = buf[0:l]

		req, _ := NewRequest(buf, false)

		req.Wait.Add(1) // 因为go routine可能还没有执行，代码就跑到最后面进行校验了

		assert.Equal(t, i+1, req.Request.SeqId, "Request SeqId是否靠谱")

		requests = append(requests, req)
	}

	go func() {
		// 客户端代码
		bc := NewBackendConn(addr, nil, "test", true)
		bc.currentSeqId = 10

		// 准备发送数据
		var i int32
		for i = 0; i < requestNum; i++ {
			fmt.Println("Sending Request to Backend Conn", i)
			bc.PushBack(requests[i])

			requests[i].Wait.Done()
		}

		// 需要等待数据返回?
		time.Sleep(time.Second * 2)

	}()

	go func() {
		// 服务器端代码
		tran, err := transport.Accept()
		if err != nil {
			log.ErrorErrorf(err, "Error: %v\n", err)
		}
		assert.NoError(t, err)

		bt := NewTBufferedFramedTransport(tran, time.Microsecond*100, 2)

		// 在当前的这个t上读写数据
		var i int32
		for i = 0; i < requestNum; i++ {
			request, err := bt.ReadFrame()
			assert.NoError(t, err)

			req, _ := NewRequest(request, false)
			assert.Equal(t, req.Request.SeqId, i+10)
			fmt.Printf("Server Got Request, and SeqNum OK, Id: %d, Frame Size: %d\n", i, len(request))

			// 回写数据
			bt.Write(request)
			bt.FlushBuffer(true)

		}

		tran.Close()
	}()

	fmt.Println("Requests Len: ", len(requests))
	for idx, r := range requests {
		r.Wait.Wait()

		// r 原始的请求
		req, _ := NewRequest(r.Response.Data, false)

		log.Printf(Green("SeqMatch[%d]: Orig: %d, Return: %d\n"), idx, req.Request.SeqId, r.Request.SeqId)
		assert.Equal(t, req.Request.SeqId, r.Request.SeqId)
	}
	log.Println("OK")
}
