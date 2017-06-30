if [ "$#" -ne 1 ]; then
    echo "Please input hostname"
    exit -1
fi

host_name=$1

# 创建目录，拷贝rpc_proxy/rpc_lb
ssh root@${host_name} "mkdir -p /usr/local/rpc_proxy/bin/"
ssh root@${host_name} "mkdir -p /usr/local/rpc_proxy/log/"

# 拷贝: rpc_lb
ssh root@${host_name} "rm -f /usr/local/rpc_proxy/bin/rpc_lb"
scp rpc_lb root@${host_name}:/usr/local/rpc_proxy/bin/rpc_lb

# 拷贝: rpc_proxy
ssh root@${host_name} "rm -f /usr/local/rpc_proxy/bin/rpc_proxy"
scp rpc_proxy root@${host_name}:/usr/local/rpc_proxy/bin/rpc_proxy

# 拷贝脚本
scp scripts/control_lb.sh    root@${host_name}:/usr/local/rpc_proxy/
scp scripts/control_proxy.sh root@${host_name}:/usr/local/rpc_proxy/

# 同时拷贝测试和线上配置
scp scripts/rpc_proxy_test.service  root@${host_name}:/lib/systemd/system/
scp scripts/rpc_proxy_online.service  root@${host_name}:/lib/systemd/system/


echo "请选择启动测试服务rpc或线上服务rpc(最好不要都启动)"
echo "ssh root@${host_name} systemctl daemon-reload"
echo "ssh root@${host_name} systemctl enable rpc_proxy_online.service"
echo "ssh root@${host_name} systemctl start rpc_proxy_online.service"
echo "---"
echo "ssh root@${host_name} systemctl daemon-reload"
echo "ssh root@${host_name} systemctl enable rpc_proxy_test.service"
echo "ssh root@${host_name} systemctl start rpc_proxy_test.service"

