//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/wfxiang08/go_thrift/thrift"

	log "github.com/wfxiang08/cyutils/utils/rolling_log"
	"io"
	"time"
)

const DEFAULT_MAX_LENGTH = 16384000

// Framed 和 Buffered各做了什么事情呢?

// TTransport接口的实现: Read/Write/Close/Flush/Open/IsOpen
// Framed就是在数据块的开头加上了数据库的长度，相当于增加了一个Frame的概念
type TBufferedFramedTransport struct {
	*thrift.TBufferedTransport
	FrameSize int     // Current remaining size of the frame. if ==0 read next frame header
	LenghR    [4]byte // 临时使用(无状态)
	LenghW    [4]byte // 临时使用(无状态)

	Buffer *bytes.Buffer // 结构体，不需要初始化

	// 安全控制
	maxLength int

	MaxBuffered int   // 单位: 请求个数
	MaxInterval int64 // 单位: nanoseconds
	nbuffered   int
	lastflush   int64 // 单位: ns(1e-9s)
}

func NewTBufferedFramedTransport(transport thrift.TTransport,
	maxInterval time.Duration,
	maxBuffered int) *TBufferedFramedTransport {

	return &TBufferedFramedTransport{
		TBufferedTransport: thrift.NewTBufferedTransport(transport, 64*1024),
		maxLength:          DEFAULT_MAX_LENGTH,
		Buffer:             bytes.NewBuffer(make([]byte, 0, 1024)),
		MaxInterval:        int64(maxInterval),
		MaxBuffered:        maxBuffered,
	}
}

// Transport上的Buffer是低频需求，不需要优化
func NewTBufferedFramedTransportMaxLength(transport thrift.TTransport,
	maxInterval time.Duration, maxBuffered int,
	maxLength int) *TBufferedFramedTransport {

	return &TBufferedFramedTransport{
		TBufferedTransport: thrift.NewTBufferedTransport(transport, 64*1024),
		maxLength:          maxLength,
		Buffer:             bytes.NewBuffer(make([]byte, 0, 1024)),
		MaxInterval:        int64(maxInterval),
		MaxBuffered:        maxBuffered,
	}
}

// 读取Frame的完整的数据，包含
func (p *TBufferedFramedTransport) ReadFrame() (frame []byte, err error) {

	if p.FrameSize != 0 {
		err = thrift.NewTTransportExceptionFromError(
			fmt.Errorf("Unexpected frame size: %d", p.FrameSize))
		return nil, err
	}
	var frameSize int

	// Reader来自transport, 中间被封装了多长
	frameSize, err = p.readFrameHeader()
	if err != nil {
		err1, ok := err.(thrift.TTransportException)

		if ok {
			err = thrift.NewTTransportException(err1.TypeId(),
				fmt.Sprintf("Frame Header Read Error: %s", err1.Error()))
		} else {
			err = thrift.NewTTransportExceptionFromError(
				fmt.Errorf("Frame Header Read Error: %s", err.Error()))
		}
		return
	}

	bytes := getSlice(frameSize, frameSize)

	// 什么时候会出现?
	// 1. 如果tcp package比较大，则容易出现package的一部分先到，另一部分随后再到
	// 2. 在同一个机器上的两个进程之间出现的概率低(Time Delay小); 跨机器访问，出现概率高
	var l int
	l, err = io.ReadFull(p.Reader, bytes)
	//	l, err = p.Reader.Read(bytes)
	if l != frameSize {
		log.Warnf(Red("<==== ReadFrame frame size: %d, Got: %d"), frameSize, l)
	}

	if err != nil {
		err1, ok := err.(thrift.TTransportException)
		if ok {
			err = thrift.NewTTransportException(err1.TypeId(),
				fmt.Sprintf("Frame Data Read Error: %s", err1.Error()))
		} else {
			err = thrift.NewTTransportExceptionFromError(
				fmt.Errorf("Frame Data Read Error: %s", err.Error()))
		}
		return nil, err
	}

	return bytes, nil

}

func (p *TBufferedFramedTransport) Read(buf []byte) (l int, err error) {

	// 1. 首先读取Frame Header
	if p.FrameSize == 0 {
		p.FrameSize, err = p.readFrameHeader()
		if err != nil {
			return
		}
	}

	// 2. 异常处理
	if p.FrameSize < len(buf) {
		frameSize := p.FrameSize
		tmp := make([]byte, p.FrameSize)
		l, err = p.Read(tmp)
		copy(buf, tmp)
		if err == nil {
			err = thrift.NewTTransportExceptionFromError(
				fmt.Errorf("Not enough frame size %d to read %d bytes",
					frameSize, len(buf)))
			return
		}
	}

	// 3. 读取剩下的数据
	got, err := p.Reader.Read(buf)
	p.FrameSize = p.FrameSize - got
	//sanity check
	if p.FrameSize < 0 {
		return 0, thrift.NewTTransportException(thrift.UNKNOWN_TRANSPORT_EXCEPTION,
			"Negative frame size")
	}
	return got, thrift.NewTTransportExceptionFromError(err)
}

func (p *TBufferedFramedTransport) ReadByte() (c byte, err error) {
	if p.FrameSize == 0 {
		p.FrameSize, err = p.readFrameHeader()
		if err != nil {
			return
		}
	}
	if p.FrameSize < 1 {
		return 0, thrift.NewTTransportExceptionFromError(
			fmt.Errorf("Not enough frame size %d to read %d bytes", p.FrameSize, 1))
	}
	c, err = p.Reader.ReadByte()
	if err == nil {
		p.FrameSize--
	}
	return
}

func (p *TBufferedFramedTransport) Write(buf []byte) (int, error) {
	// 直接写入Buffer
	n, err := p.Buffer.Write(buf)
	if n != len(buf) {
		log.Errorf("TBufferedFramedTransport#Write Error")
	}
	return n, thrift.NewTTransportExceptionFromError(err)
}

func (p *TBufferedFramedTransport) WriteByte(c byte) error {
	return p.Buffer.WriteByte(c)
}

func (p *TBufferedFramedTransport) WriteString(s string) (n int, err error) {
	return p.Buffer.WriteString(s)
}
func (p *TBufferedFramedTransport) Flush() error {
	// 兼容默认的策略
	return p.FlushBuffer(true)
}

func (p *TBufferedFramedTransport) flushTransport(force bool) error {
	if p.nbuffered > 0 && (force || p.needFlush()) {
		// 这个如何控制呢?
		if err := p.Writer.Flush(); err != nil {
			log.ErrorErrorf(err, "FlushTransport Error, %v", err)
			return err
		}
		p.nbuffered = 0
		p.lastflush = time.Now().UnixNano()
	}
	return nil
}

//
// 先写入数据，然后再Flush Transport
//
func (p *TBufferedFramedTransport) FlushBuffer(force bool) error {
	size := p.Buffer.Len()

	// 没有有效的数据，直接返回
	if size == 0 {
		return nil
	}
	// TODO: 待优化
	force = true

	// 1. 将p.buf的大小以BigEndian模式写入: buf中
	buf := p.LenghW[:4]
	binary.BigEndian.PutUint32(buf, uint32(size))

	//	log.Printf("----> Frame Size: %d, %v\n", size, buf)
	// 然后transport中先写入: 长度信息
	_, err := p.Writer.Write(buf)
	if err != nil {
		return thrift.NewTTransportExceptionFromError(err)
	}

	// log.Printf("Flushing buffer with size: %d to backend", size)

	// 2. 然后继续写入p.buf中的数据
	if size > 0 {
		var (
			n   int64
			err error
		)
		// 如果 err == io.ErrShortWrite， p.Writer中也有buffer, 因此可以不用考虑异常
		if n, err = p.Buffer.WriteTo(p.Writer); err != nil {
			log.ErrorErrorf(err, "Error Flushing Expect Write: %d, but %d\n",
				size, n)
			return thrift.NewTTransportExceptionFromError(err)
		}
		if n < int64(size) {
			log.Printf(Red("Buffer Write Not Finished"))
		}
	}

	p.nbuffered++

	// Buffer重新开始处理数据
	p.Buffer.Reset()

	// Flush Buffer
	return p.flushTransport(force)
}

func (p *TBufferedFramedTransport) readFrameHeader() (int, error) {
	buf := p.LenghR[:4]

	if _, err := io.ReadFull(p.Reader, buf); err != nil {
		return 0, err
	}
	size := int(binary.BigEndian.Uint32(buf))
	if size < 0 || size > p.maxLength {
		return 0, thrift.NewTTransportException(thrift.UNKNOWN_TRANSPORT_EXCEPTION,
			fmt.Sprintf("Incorrect frame size (%d)", size))
	}
	return size, nil
}

func (p *TBufferedFramedTransport) needFlush() bool {
	if p.nbuffered != 0 {
		// request个数 buffer到一定数量
		if p.nbuffered >= p.MaxBuffered {
			return true
		}

		// 或者间隔到达一定时间，则flush
		if time.Now().UnixNano()-p.lastflush > p.MaxInterval {
			return true
		}
	}
	return false
}
