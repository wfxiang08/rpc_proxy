//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	"fmt"
	thrift "github.com/wfxiang08/go_thrift/thrift"
)

//
// 生成Thrift格式的Exception Message
//
func GetServiceNotFoundData(req *Request) []byte {
	req.Response.TypeId = thrift.EXCEPTION

	// 构建thrift的Transport
	transport := thrift.NewTMemoryBufferLen(100)
	protocol := thrift.NewTBinaryProtocolTransport(transport)

	// 构建一个Message, 写入Exception
	msg := fmt.Sprintf("Service: %s Not Found", req.Service)
	exc := thrift.NewTApplicationException(thrift.UNKNOWN_APPLICATION_EXCEPTION, msg)

	protocol.WriteMessageBegin(req.Request.Name, thrift.EXCEPTION, req.Request.SeqId)
	exc.Write(protocol)
	protocol.WriteMessageEnd()
	protocol.Flush()

	bytes := transport.Bytes()
	return bytes
}

func GetWorkerNotFoundData(req *Request, module string) []byte {
	req.Response.TypeId = thrift.EXCEPTION

	// 构建thrift的Transport
	transport := thrift.NewTMemoryBufferLen(100)
	protocol := thrift.NewTBinaryProtocolTransport(transport)

	// 构建一个Message, 写入Exception
	msg := fmt.Sprintf("Worker FOR %s#%s.%s Not Found", module, req.Service, req.Request.Name)
	exc := thrift.NewTApplicationException(thrift.INTERNAL_ERROR, msg)

	protocol.WriteMessageBegin(req.Request.Name, thrift.EXCEPTION, req.Request.SeqId)
	exc.Write(protocol)
	protocol.WriteMessageEnd()

	bytes := transport.Bytes()
	return bytes
}

func GetThriftException(req *Request, module string) []byte {
	req.Response.TypeId = thrift.EXCEPTION

	// 构建thrift的Transport
	transport := thrift.NewTMemoryBufferLen(256)
	protocol := thrift.NewTBinaryProtocolTransport(transport)

	msg := fmt.Sprintf("Module: %s, Service: %s, Method: %s, Error: %v", module, req.Service, req.Request.Name, req.Response.Err)

	// 构建一个Message, 写入Exception
	exc := thrift.NewTApplicationException(thrift.INTERNAL_ERROR, msg)

	// 注意消息的格式
	protocol.WriteMessageBegin(req.Request.Name, thrift.EXCEPTION, req.Request.SeqId)
	exc.Write(protocol)
	protocol.WriteMessageEnd()

	bytes := transport.Bytes()
	return bytes
}

//
// 解析Thrift数据的Message Header
//
func ParseThriftMsgBegin(msg []byte) (name string, typeId thrift.TMessageType, seqId int32, err error) {
	transport := thrift.NewTMemoryBufferLen(256)
	transport.Write(msg)
	protocol := thrift.NewTBinaryProtocolTransport(transport)
	name, typeId, seqId, err = protocol.ReadMessageBegin()
	return
}
