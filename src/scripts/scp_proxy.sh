#!/usr/bin/env bash
if [ "$#" -ne 1 ]; then
    echo "Please input hostname"
    exit -1
fi

host_name=$1

ssh root@${host_name} "mkdir -p /usr/local/rpc_proxy/bin/"
ssh root@${host_name} "mkdir -p /usr/local/rpc_proxy/log/"

ssh root@${host_name} "rm -f /usr/local/rpc_proxy/bin/service_rpc_proxy"
scp service_rpc_proxy root@${host_name}:/usr/local/rpc_proxy/bin/service_rpc_proxy
scp scripts/proxy-config.online.ini root@${host_name}:/usr/local/rpc_proxy/proxy-config.online.ini

ssh root@${host_name} "chown -R worker.worker /usr/local/rpc_proxy/"


# 拷贝systemctl
scp scripts/rpc_proxy_online.service root@${host_name}:/lib/systemd/system/rpc_proxy_online.service

# 启动服务
ssh root@${host_name} "systemctl daemon-reload"
# ssh root@${host_name} "if systemctl is-active rpc_proxy_online.service; then systemctl reload rpc_proxy_online; else systemctl start rpc_proxy_online; fi"
ssh root@${host_name} "systemctl restart rpc_proxy_online.service"
