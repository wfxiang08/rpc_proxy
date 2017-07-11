#!/usr/bin/env bash
go build -ldflags "-X main.buildDate=`date +%Y%m%d%H%M%S` -X main.gitVersion=`git rev-parse HEAD`" cmds/service_rpc_proxy.go