//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	"bytes"
)

// Memory buffer-based implementation of the TTransport interface.
type TMemoryBuffer struct {
	// *bytes.Buffer 和 bytes.Buffer不一样， 使用起来一样，但是: 前者似乎可以定制，可以指定buffer的初始大小
	*bytes.Buffer
	size int
}

func NewTMemoryBuffer() *TMemoryBuffer {
	return &TMemoryBuffer{Buffer: &bytes.Buffer{}, size: 0}
}

func NewTMemoryBufferLen(size int) *TMemoryBuffer {
	buf := make([]byte, 0, size)
	return &TMemoryBuffer{Buffer: bytes.NewBuffer(buf), size: size}
}

//
// 直接利用现成的Buffer, 由于size不能控制，因此只能拷贝一份出来
//
func NewTMemoryBufferWithBuf(buf []byte) *TMemoryBuffer {
	// buf作为bytes.Buffer的已有的数据，因此需要特别注意
	return &TMemoryBuffer{Buffer: bytes.NewBuffer(buf), size: len(buf)}
}

func (p *TMemoryBuffer) IsOpen() bool {
	return true
}

func (p *TMemoryBuffer) Open() error {
	return nil
}

func (p *TMemoryBuffer) Close() error {
	p.Buffer.Reset()
	return nil
}

// Flushing a memory buffer is a no-op
func (p *TMemoryBuffer) Flush() error {
	return nil
}
