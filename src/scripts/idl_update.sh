#!/usr/bin/env bash
thrift -r --gen go:thrift_import="github.com/wfxiang08/go_thrift/thrift" scripts/rpc_thrift.services.thrift
rm -rf src/rpc_thrift
mv gen-go/rpc_thrift src/rpc_thrift
rm -rf gen-go