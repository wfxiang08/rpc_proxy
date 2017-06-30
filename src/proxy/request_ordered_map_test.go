//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

//
// go test proxy -v -run "TestRequestMap"
//
func TestRequestMap(t *testing.T) {

	requestMap, _ := NewRequestMap(20)

	var i int32
	for i = 0; i < 10; i++ {
		request := NewPingRequest()
		request.Start += int64(1000000 * 5 * (i - 5))
		request.Response.SeqId = i

		requestMap.Add(request.Response.SeqId, request)
	}

	assert.Equal(t, 10, requestMap.Len())

	sid, request, ok := requestMap.PeekOldest()
	fmt.Printf("Request: %s\n, Sid: %d\n", request.Request.Name, request.Response.SeqId)
	assert.Equal(t, int32(0), int32(sid))
	assert.True(t, ok)

	expired := microseconds()
	for true {
		sid, request, ok := requestMap.PeekOldest()
		if ok && request.Start < expired {
			requestMap.Remove(sid)
		} else {
			break
		}
	}

	assert.Equal(t, 4, requestMap.Len())

	requests := requestMap.Purge()
	assert.Equal(t, 4, len(requests))

	for _, request := range requests {
		fmt.Printf("Request: %s\n, Sid: %d\n", request.Request.Name, request.Response.SeqId)
	}
}
