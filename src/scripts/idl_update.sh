#!/usr/bin/env bash
thrift -r --gen go:thrift_import="github.com/wfxiang08/go_thrift/thrift" scripts/RpcThrift.Services.thrift