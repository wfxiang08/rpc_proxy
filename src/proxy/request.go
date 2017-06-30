//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	"encoding/binary"
	"errors"
	"fmt"
	thrift "github.com/wfxiang08/go_thrift/thrift"
	log "github.com/wfxiang08/cyutils/utils/rolling_log"
	"io"
	"strings"
	"sync"
)

type Dispatcher interface {
	Dispatch(r *Request) error
}

const (
	// 不能使用负数
	MESSAGE_TYPE_HEART_BEAT thrift.TMessageType = 20
	MESSAGE_TYPE_STOP       thrift.TMessageType = 21
)

type Request struct {
	Service      string // 服务
	ProxyRequest bool   // Service是否出现在Request.Data中，默认为true, 但是心跳等信号中没有service

	// 原始的数据(虽然拷贝有点点效率低，但是和zeromq相比也差不多)
	Request struct {
		Name     string
		TypeId   thrift.TMessageType
		SeqId    int32
		Data     []byte
		DataOrig []byte
	}

	Start int64

	// 返回的数据类型
	Response struct {
		Data   []byte
		Err    error
		SeqId  int32 // -1保留，表示没有对应的SeqNum
		TypeId thrift.TMessageType
	}

	Wait sync.WaitGroup
}

//
// 给定一个thrift message，构建一个Request对象
//
func NewRequest(data []byte, serviceInReq bool) (*Request, error) {
	request := &Request{
		ProxyRequest: serviceInReq,
		Start:        microseconds(),
	}
	request.Request.Data = data
	err := request.DecodeRequest()

	if err != nil {
		return nil, err
	} else {
		return request, nil
	}

}

//
// 利用自身信息生成 timeout Error(注意: 其中的SeqId必须为r.Request.SeqId(它不是从后台返回，不会进行SeqId的replace)
//
func (r *Request) NewTimeoutError() error {
	return errors.New(fmt.Sprintf("Timeout Exception, %s.%s.%d",
		r.Service, r.Request.Name, r.Request.SeqId))
}

func (r *Request) NewInvalidResponseError(method string, module string) error {
	return errors.New(fmt.Sprintf("[%s]Invalid Response Exception, %s.%s.%d, Method Ret: %s",
		module, r.Service, r.Request.Name, r.Request.SeqId, method))
}

//
// 从Request.Data中读取出 Request的Name, TypeId, SeqId
// RequestName可能和thrift package中的name不一致，Service部分从Name中剔除
//
func (r *Request) DecodeRequest() error {
	var err error
	transport := NewTMemoryBufferWithBuf(r.Request.Data)
	protocol := thrift.NewTBinaryProtocolTransport(transport)

	// 如果数据异常，则直接报错
	r.Request.Name, r.Request.TypeId, r.Request.SeqId, err = protocol.ReadMessageBegin()
	if err != nil {
		return err
	}

	// 参考 ： TMultiplexedProtocol
	idx := strings.Index(r.Request.Name, thrift.MULTIPLEXED_SEPARATOR)
	if idx == -1 {
		r.Service = ""
	} else {
		r.Service = r.Request.Name[0:idx]
		r.Request.Name = r.Request.Name[idx+1 : len(r.Request.Name)]
	}
	return nil
}

//
// 将Request中的SeqNum进行替换（修改Request部分的数据)
//
func (r *Request) ReplaceSeqId(newSeq int32) {
	if r.Request.Data != nil {
		//		log.Printf(Green("Replace SeqNum: %d --> %d"), r.Request.SeqId, newSeq)
		if r.Response.SeqId != 0 {
			log.Errorf("Unexpected Response SedId")
		}
		r.Response.SeqId = newSeq

		start := 0

		if r.ProxyRequest {
			start = len(r.Service)
		}
		if start > 0 {
			start += 1 // ":"
			//			log.Printf("Service: %s, Name: %s\n", r.Service, r.Request.Name)
		}
		transport := NewTMemoryBufferWithBuf(r.Request.Data[start:start])
		protocol := thrift.NewTBinaryProtocolTransport(transport)
		protocol.WriteMessageBegin(r.Request.Name, r.Request.TypeId, newSeq)

		if start > 0 {
			r.Request.DataOrig = r.Request.Data
		}
		// 将service从name中剥离出去
		r.Request.Data = r.Request.Data[start:len(r.Request.Data)]

	} else {
		log.Errorf("ReplaceSeqId called on processed Data")
	}
}

func (r *Request) Recycle() {
	var sliceId uintptr = 0
	// 将其中的Buffer归还(returnSlice)
	if r.Request.DataOrig != nil {
		returnSlice(r.Request.DataOrig)
		sliceId = getSliceId(r.Request.DataOrig)

		r.Request.DataOrig = nil
		r.Request.Data = nil
	} else if r.Request.Data != nil {
		sliceId = getSliceId(r.Request.Data)
		returnSlice(r.Request.Data)
		r.Request.Data = nil
	}
	if r.Response.Data != nil {
		if sliceId != getSliceId(r.Response.Data) {
			returnSlice(r.Response.Data)
		}
		r.Response.Data = nil
	}
}

func (r *Request) RestoreSeqId() {
	if r.Response.Data != nil {
		// e = p.WriteI32(seqId)
		// i32 + (i32 + len(str)) + i32[SeqId]
		// 直接按照TBinaryProtocol协议，修改指定位置的数据: SeqId
		startIdx := 4 + 4 + len(r.Request.Name)
		v := r.Response.Data[startIdx : startIdx+4]
		binary.BigEndian.PutUint32(v, uint32(r.Request.SeqId))

		//		transport := NewTMemoryBufferWithBuf(r.Response.Data[0:0])
		//		protocol := thrift.NewTBinaryProtocolTransport(transport)

		//		// 切换回原始的SeqId
		//		// r.Response.TypeId 和 r.Request.TypeId可能不一样，要以Response为准
		//		protocol.WriteMessageBegin(r.Request.Name, r.Response.TypeId, r.Request.SeqId)
	}
}

//
// 给定thrift Message, 解码出: typeId, seqId
//
func DecodeThriftTypIdSeqId(data []byte) (typeId thrift.TMessageType, method string, seqId int32, err error) {

	// 解码typeId
	if len(data) < 4 {
		err = thrift.NewTProtocolException(io.ErrUnexpectedEOF)
		return
	}
	size := int32(binary.BigEndian.Uint32(data[0:4]))
	typeId = thrift.TMessageType(size & 0x0ff)

	// 解码name的长度，并且跳过name
	if len(data) < 8 {
		err = thrift.NewTProtocolException(io.ErrUnexpectedEOF)
		return
	}
	size = int32(binary.BigEndian.Uint32(data[4:8]))
	if len(data) < 12+int(size) {
		err = thrift.NewTProtocolException(io.ErrUnexpectedEOF)
		return
	}

	method = string(data[8 : 8+size])
	// 解码seqId
	seqId = int32(binary.BigEndian.Uint32(data[8+size : 12+size]))

	//	transport := NewTMemoryBufferWithBuf(data)
	//	protocol := thrift.NewTBinaryProtocolTransport(transport)

	//	_, typeId, seqId, err = protocol.ReadMessageBegin()
	return
}
