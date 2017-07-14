#!/usr/bin/env bash
if [ "$#" -ne 1 ]; then
    echo "Please input hostname"
    exit -1
fi

host_name=$1

ssh root@${host_name} "mkdir -p /usr/local/rpc_proxy/bin/"
ssh root@${host_name} "mkdir -p /usr/local/rpc_proxy/log/"

ssh root@${host_name} "rm -f /usr/local/rpc_proxy/bin/service_rpc_lb"
scp service_rpc_lb root@${host_name}:/usr/local/rpc_proxy/bin/service_rpc_lb
ssh root@${host_name} "chown -R worker.worker /usr/local/rpc_proxy/"

