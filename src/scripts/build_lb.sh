#!/usr/bin/env bash
go build -ldflags "-X main.buildDate=`date +%Y%m%d%H%M%S` -X main.gitVersion=`git -C .. rev-parse HEAD`" cmds/service_rpc_lb.go
