//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

//
// go test proxy -v -run "TestMemoryAlloc"
//
func TestMemoryAlloc(t *testing.T) {
	v1 := getSlice(100, 100)
	assert.True(t, len(v1) == 100, cap(v1) == 1024)

	v2 := v1[4:5]

	// Slice一旦操作之后，其记录的信息就不完整了, 根据v2无法获取v1的信息
	fmt.Printf("V1: %d, %d, V2: %d, %d\n", len(v1), cap(v1), len(v2), cap(v2))
	assert.False(t, returnSlice(v2))
	assert.True(t, returnSlice(v1))

	// 同一个slice不要归还多次

}
