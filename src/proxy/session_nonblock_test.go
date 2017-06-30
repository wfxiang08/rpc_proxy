//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	"github.com/stretchr/testify/assert"

	"testing"
)

//
// go test proxy -v -run "TestSessionNonBlock"
//
func TestSessionNonBlock(t *testing.T) {

	// result = client.correct_typo_simple("HELLO")
	typoHello := []byte{128, 1, 0, 1, 0, 0, 0, 19, 99, 111, 114, 114, 101, 99,
		116, 95, 116, 121, 112, 111, 95, 115, 105, 109, 112, 108, 101, 0, 0, 3,
		239, 0, 5, 72, 69, 76, 76, 79, 0}
	s := &NonBlockSession{}
	d := &testDispatcher1{}

	// 确保SessionNonBlock创建的Request满足要求
	r, err1 := NewRequest(typoHello, false)
	s.handleRequest(r, d)
	assert.False(t, d.Request.ProxyRequest)
}

type testDispatcher1 struct {
	Request *Request
}

func (d *testDispatcher1) Dispatch(r *Request) error {
	d.Request = r
	return nil
}
