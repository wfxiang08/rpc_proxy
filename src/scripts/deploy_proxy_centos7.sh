if [ "$#" -ne 1 ]; then
    echo "Please input hostname"
    exit -1
fi


host_name=$1

if [[ $host_name == "localhost" ]]; then
    echo "deply proxy to local host"
    mkdir -p /usr/local/rpc_proxy/bin/
    mkdir -p /usr/local/rpc_proxy/log/
    rm -f /usr/local/rpc_proxy/bin/rpc_lb
    rm -f /usr/local/rpc_proxy/bin/rpc_proxy
    cp rpc_lb /usr/local/rpc_proxy/bin/rpc_lb
    cp rpc_proxy /usr/local/rpc_proxy/bin/rpc_proxy
    cp scripts/config.ucloud-online.ini  /usr/local/rpc_proxy/config.online.ini
    cp scripts/config.ucloud-test.ini  /usr/local/rpc_proxy/config.test.ini
    cp scripts/rpc_proxy_test.service  /lib/systemd/system/
    cp scripts/rpc_proxy_online.service /lib/systemd/system/

    echo "请选择启动测试服务rpc或线上服务rpc(最好不要都启动)"
    echo "systemctl daemon-reload"
    echo "systemctl enable rpc_proxy_online.service"
    echo "systemctl start rpc_proxy_online.service"
    echo "---"
    echo "systemctl daemon-reload"
    echo "systemctl enable rpc_proxy_test.service"
    echo "systemctl start rpc_proxy_test.service"

else
    # 创建目录，拷贝rpc_proxy/rpc_lb
    echo "ssh root@${host_name} mkdir -p /usr/local/rpc_proxy/bin/"
    ssh root@${host_name} "mkdir -p /usr/local/rpc_proxy/bin/"
    ssh root@${host_name} "mkdir -p /usr/local/rpc_proxy/log/"

    # 拷贝: rpc_lb
    echo "ssh root@${host_name} rm -f /usr/local/rpc_proxy/bin/rpc_lb"
    ssh root@${host_name} "rm -f /usr/local/rpc_proxy/bin/rpc_lb"

    echo "scp rpc_lb root@${host_name}:/usr/local/rpc_proxy/bin/rpc_lb"
    scp rpc_lb root@${host_name}:/usr/local/rpc_proxy/bin/rpc_lb

    # 拷贝: rpc_proxy
    echo "ssh root@${host_name} rm -f /usr/local/rpc_proxy/bin/rpc_proxy"
    ssh root@${host_name} "rm -f /usr/local/rpc_proxy/bin/rpc_proxy"

    echo "scp rpc_lb root@${host_name}:/usr/local/rpc_proxy/bin/rpc_proxy"
    scp rpc_proxy root@${host_name}:/usr/local/rpc_proxy/bin/rpc_proxy

    # 拷贝脚本
    scp scripts/control_lb.sh    root@${host_name}:/usr/local/rpc_proxy/
    scp scripts/control_proxy.sh root@${host_name}:/usr/local/rpc_proxy/

    # 同时拷贝测试和线上配置
    scp scripts/config.ucloud-online.ini  root@${host_name}:/usr/local/rpc_proxy/config.online.ini
    scp scripts/config.ucloud-test.ini  root@${host_name}:/usr/local/rpc_proxy/config.test.ini
    scp scripts/rpc_proxy_test.service  root@${host_name}:/lib/systemd/system/
    scp scripts/rpc_proxy_online.service  root@${host_name}:/lib/systemd/system/


    echo "请选择启动测试服务rpc或线上服务rpc(最好不要都启动)"
    echo "ssh root@${host_name} systemctl daemon-reload"
    echo "ssh root@${host_name}systemctl enable rpc_proxy_online.service"
    echo "ssh root@${host_name} systemctl start rpc_proxy_online.service"
    echo "---"
    echo "ssh root@${host_name} systemctl daemon-reload"
    echo "ssh root@${host_name} systemctl enable rpc_proxy_test.service"
    echo "ssh root@${host_name} systemctl start rpc_proxy_test.service"
fi

