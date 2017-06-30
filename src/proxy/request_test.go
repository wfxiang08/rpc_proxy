//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	"fmt"
	thrift "github.com/wfxiang08/go_thrift/thrift"
	"github.com/stretchr/testify/assert"
	"testing"
)

func fakeData(name string, typeId thrift.TMessageType, seqId int32, buf []byte) int {
	transport := NewTMemoryBufferWithBuf(buf)
	protocol := thrift.NewTBinaryProtocolTransport(transport)

	// 切换回原始的SeqId
	protocol.WriteMessageBegin(name, typeId, seqId)
	return transport.Buffer.Len()

}

//
// go test proxy -v -run "TestRequest"
//
func TestRequest(t *testing.T) {

	a := make([]byte, 1000)
	c := a
	d := a
	var e []byte
	fmt.Println("E: ", getSliceId(e), "A: ", getSliceId(c))
	assert.True(t, getSliceId(c) == getSliceId(d))

	data := make([]byte, 1000, 1000)
	size := fakeData("demo:hello", thrift.CALL, 0, data[0:0])
	data = data[0:size]

	r, _ := NewRequest(data, true)

	assert.Equal(t, "demo", r.Service)
	assert.Equal(t, "hello", r.Request.Name)

	//	fmt.Printf("Name: %s, SeqId: %d, TypeId: %d\n", r.Request.Name,
	//		r.Request.SeqId, r.Request.TypeId)

	var newSeqId int32 = 10
	r.ReplaceSeqId(newSeqId)

	_, _, seqId1, _ := DecodeThriftTypIdSeqId(r.Request.Data)
	assert.Equal(t, newSeqId, seqId1) // r.Request.Data中的id被替换成功

	r.Response.Data = r.Request.Data
	r.RestoreSeqId()

	// 恢复正常
	_, _, seqId2, _ := DecodeThriftTypIdSeqId(r.Response.Data)

	//	fmt.Printf("Reqeust SeqId: %d, %d\n", r.Request.SeqId, seqId2)
	assert.Equal(t, 0, int(seqId2))

}
