if [ "$#" -ne 1 ]; then
    echo "Please input hostname"
    exit -1
fi

host_name=$1

echo "ssh root@${host_name} ""rm -f /usr/local/rpc_proxy/bin/rpc_lb"""
ssh root@${host_name} "rm -f /usr/local/rpc_proxy/bin/rpc_lb"

echo "scp rpc_lb root@${host_name}:/usr/local/rpc_proxy/bin/rpc_lb"
scp rpc_lb root@${host_name}:/usr/local/rpc_proxy/bin/rpc_lb

