//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	"fmt"
	thrift "github.com/wfxiang08/go_thrift/thrift"
	log "github.com/wfxiang08/cyutils/utils/rolling_log"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func init() {
	log.SetLevel(log.LEVEL_INFO)
	log.SetFlags(log.Flags() | log.Lshortfile)
}

type fakeServer struct {
}

func (p *fakeServer) Dispatch(r *Request) error {
	log.Printf("Request SeqId: %d, MethodName: %s\n", r.Request.SeqId, r.Request.Name)
	r.Wait.Add(1)
	go func() {
		time.Sleep(time.Millisecond)
		r.Response.Data = []byte(string(r.Request.Data))

		typeId, _, seqId, _ := DecodeThriftTypIdSeqId(r.Response.Data)
		log.Printf(Green("TypeId: %d, SeqId: %d\n"), typeId, seqId)
		r.Wait.Done()
	}()
	//	r.RestoreSeqId()
	//	r.Wait.Done()
	return nil
}

//
// go test proxy -v -run "TestSession"
//
func TestSession(t *testing.T) {
	// 作为一个Server
	transport, err := thrift.NewTServerSocket("127.0.0.1:0")
	err = transport.Open() // 打开Transport
	defer transport.Close()

	err = transport.Listen() // 开始监听
	assert.NoError(t, err)

	addr := transport.Addr().String()
	fmt.Println("Addr: ", addr)

	// 1. Fake Requests
	var requestNum int32 = 10
	requests := make([]*Request, 0, requestNum)
	var i int32
	for i = 0; i < requestNum; i++ {
		buf := make([]byte, 100, 100)
		l := fakeData("Hello", thrift.CALL, i+1, buf[0:0])
		buf = buf[0:l]

		req, _ := NewRequest(buf, true)

		req.Wait.Add(1) // 因为go routine可能还没有执行，代码就跑到最后面进行校验了

		assert.Equal(t, i+1, req.Request.SeqId, "Request SeqId是否靠谱")

		requests = append(requests, req)
	}

	// 2. 将请求交给BackendConn
	go func() {
		// 模拟请求:
		// 客户端代码
		bc := NewBackendConn(addr, nil, "test", true)
		bc.currentSeqId = 10

		// 上线 BackendConn
		bc.IsConnActive.Set(true)

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

	server := &fakeServer{}
	go func() {
		// 服务器端代码
		tran, err := transport.Accept()
		defer tran.Close()
		if err != nil {
			log.ErrorErrorf(err, "Error: %v\n", err)
		}
		assert.NoError(t, err)

		// 建立一个长连接, 同上面的: NewBackendConn通信
		session := NewSession(tran, "", true)
		session.Serve(server, 6)

		time.Sleep(time.Second * 2)
	}()

	for i = 0; i < requestNum; i++ {
		fmt.Println("===== Before Wait")
		requests[i].Wait.Wait()
		fmt.Println("===== Before After Wait")

		log.Printf("Request: %d, .....", i)
		assert.Equal(t, len(requests[i].Request.Data), len(requests[i].Response.Data))
	}
}
