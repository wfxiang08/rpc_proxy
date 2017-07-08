//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	"github.com/wfxiang08/go_thrift/thrift"
	"github.com/wfxiang08/thrift_rpc_base/rpcthrift/services"
)

//
// 生成Thrift格式的Exception Message
//
func HandlePingRequest(req *Request) {
	req.Response.Data = req.Request.Data
}

func HandleProxyPingRequest(req *Request) {
	transport := NewTMemoryBufferLen(30)
	protocol := thrift.NewTBinaryProtocolTransport(transport)
	protocol.WriteMessageBegin("ping", thrift.REPLY, req.Request.SeqId)
	result := services.NewRpcServiceBasePingResult()
	result.Write(protocol)
	protocol.WriteMessageEnd()
	protocol.Flush()

	req.Response.Data = transport.Bytes()
}

func NewPingRequest() *Request {
	// 构建thrift的Transport

	transport := NewTMemoryBufferLen(30)
	protocol := thrift.NewTBinaryProtocolTransport(transport)
	protocol.WriteMessageBegin("ping", MESSAGE_TYPE_HEART_BEAT, 0)
	protocol.WriteMessageEnd()
	protocol.Flush()

	r := &Request{}
	// 告诉Request, Data中不包含service，在ReplaceSeqId时不需要特别处理
	r.ProxyRequest = false
	r.Start = microseconds()
	r.Request.Data = transport.Bytes()
	r.Request.Name = "ping"
	r.Request.SeqId = 0 // SeqId在这里无效，因此设置为0
	r.Request.TypeId = MESSAGE_TYPE_HEART_BEAT
	return r
}

func NewStopConfirmRequest() *Request {
	// 构建thrift的Transport

	transport := NewTMemoryBufferLen(30)
	protocol := thrift.NewTBinaryProtocolTransport(transport)
	protocol.WriteMessageBegin("stop_confirm", MESSAGE_TYPE_STOP_CONFIRM, 0)
	protocol.WriteMessageEnd()
	protocol.Flush()

	r := &Request{}
	// 告诉Request, Data中不包含service，在ReplaceSeqId时不需要特别处理
	r.ProxyRequest = false
	r.Start = microseconds()
	r.Request.Data = transport.Bytes()
	r.Request.Name = "stop_confirm"
	r.Request.SeqId = 0 // SeqId在这里无效，因此设置为0
	r.Request.TypeId = MESSAGE_TYPE_STOP_CONFIRM
	return r
}