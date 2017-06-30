//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

//
// go test proxy -v -run "TestEndpoint"
//
func TestEndpoint(t *testing.T) {
	//  {"service":"Service","service_id":"ServiceId","frontend":"Frontend","deploy_path":"DeployPath","hostname":"Hostname","start_time":"StartTime"}

	// 1. 正常的Marshal & Unmarshal
	endpoint := &ServiceEndpoint{
		Service:    "Service",
		ServiceId:  "ServiceId",
		Frontend:   "Frontend",
		DeployPath: "DeployPath",
		Hostname:   "Hostname",
		StartTime:  "StartTime",
	}

	data, _ := json.Marshal(endpoint)
	fmt.Println("Endpoint: ", string(data))

	assert.True(t, true)

	// 2. 缺少字段时的Unmarshal(缺少的字段为空)
	data21 := []byte(`{"service":"Service","service_id":"ServiceId","frontend":"Frontend"}`)

	endpoint2 := &ServiceEndpoint{}
	err2 := json.Unmarshal(data21, endpoint2)
	assert.True(t, err2 == nil)

	fmt.Println("Error2: ", err2)
	data22, _ := json.Marshal(endpoint2)
	fmt.Println("Endpoint2: ", string(data22))

	// 3. 字段多的情况下的Unmarshal(多余的字段直接忽略)
	data31 := []byte(`{"service":"Service", "serviceA":"AService","service_id":"ServiceId","frontend":"Frontend"}`)
	endpoint3 := &ServiceEndpoint{}
	err3 := json.Unmarshal(data31, endpoint3)
	assert.True(t, err3 == nil)
	fmt.Println("Error3: ", err3)
	data32, _ := json.Marshal(endpoint3)
	fmt.Println("Endpoint3: ", string(data32))

}
