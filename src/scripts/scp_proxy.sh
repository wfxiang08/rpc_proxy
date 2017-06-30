if [ "$#" -ne 1 ]; then
    echo "Please input hostname"
    exit -1
fi

host_name=$1

echo "ssh root@${host_name} ""rm -f /usr/local/rpc_proxy/bin/rpc_proxy"""
ssh root@${host_name} "rm -f /usr/local/rpc_proxy/bin/rpc_proxy"

echo "scp rpc_proxy root@${host_name}:/usr/local/rpc_proxy/bin/rpc_proxy"
scp rpc_proxy root@${host_name}:/usr/local/rpc_proxy/bin/rpc_proxy


# 重启Proxy
# ssh root@${host_name} "/usr/local/rpc_proxy/control_proxy.sh stop"
# ssh root@${host_name} "/usr/local/rpc_proxy/control_proxy.sh start"

ssh root@${host_name} "stop rpc_proxy"
ssh root@${host_name} "start rpc_proxy"
