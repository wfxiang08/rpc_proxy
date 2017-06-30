#!/usr/bin/env bash
if [ "$#" -ne 1 ]; then
    echo "Please input hostname"
    exit -1
fi

host_name=$1

ssh root@${host_name} "rm -f /usr/local/rpc_proxy/bin/rpc_proxy"
scp rpc_proxy root@${host_name}:/usr/local/rpc_proxy/bin/rpc_proxy
