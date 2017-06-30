//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
	"unsafe"
)

//
// go test proxy -v -run "TestMemoryBuffer"
//
func TestMemoryBuffer(t *testing.T) {

	var buf *TMemoryBuffer

	// Case 1:
	buf = NewTMemoryBuffer()
	// 长度
	assert.Equal(t, 0, buf.Buffer.Len())
	assert.Equal(t, 0, buf.Len())

	buf.Write([]byte("hello"))
	buf.Write([]byte(" world"))

	// Bytes()的用法
	assert.True(t, bytes.Equal(buf.Bytes(), []byte("hello world")))

	// Case 2:
	// 这个size只是给了buf一个默认的cap, 除此之外没有其他的用处
	buf = NewTMemoryBufferLen(10)
	buf.Write([]byte("1234567890abcdefg"))
	assert.Equal(t, len("1234567890abcdefg"), buf.Len())
	assert.Equal(t, len("1234567890abcdefg"), buf.Buffer.Len())

	// Case 3.1
	byteBuf := make([]byte, 10, 10)
	buf = NewTMemoryBufferWithBuf(byteBuf[0:0]) // 注意: 0:0的意义
	buf.Write([]byte("1234567890"))

	// buf没有resize, 则byteBuf和buf中的buffer一致
	assert.True(t, bytes.Equal(byteBuf, buf.Bytes()))

	// 如果slice大小不够，则会重新创建，从而到只: byteBuf和buf中的buffer不一样
	buf.Write([]byte("123456789012"))
	assert.False(t, bytes.Equal(byteBuf, buf.Bytes()))

	// Case 3.2
	byteBuf = make([]byte, 11, 30)
	copy(byteBuf, []byte("hello world"))

	buf = NewTMemoryBufferWithBuf(byteBuf)
	buf.Write([]byte("Hi"))

	// 因为slice的长度限制了byteBuf
	assert.NotEqual(t, "hello worldHi", string(byteBuf))
	// 潜在的slice是相同的
	assert.Equal(t, "hello worldHi", string(copySlice(byteBuf, buf.Len())))

}

func copySlice(s []byte, sl int) []byte {
	var b []byte

	h := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	h.Data = (*reflect.SliceHeader)(unsafe.Pointer(&s)).Data
	h.Len = sl
	h.Cap = len(s)

	return b
}
