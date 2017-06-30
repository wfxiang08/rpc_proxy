//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	color "github.com/fatih/color"
)

// 警告信息采用红色显示
var Red = color.New(color.FgRed).SprintFunc()

// 新增服务等采用绿色显示
var Green = color.New(color.FgGreen).SprintFunc()

var Magenta = color.New(color.FgMagenta).SprintFunc()
var Cyan = color.New(color.FgCyan).SprintFunc()

var Blue = color.New(color.FgBlue).SprintFunc()
