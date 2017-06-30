#!/bin/bash


# 由于proxy基本上是每个机器上有一个拷贝，并且和项目无关，因此放在一个固定地方
BASE_DIR=/usr/local/rpc_proxy/
mkdir -p ${BASE_DIR}log

cd ${BASE_DIR}

app=${BASE_DIR}bin/rpc_proxy
conf=${BASE_DIR}config.ini

logfile=${BASE_DIR}log/proxy.log
pidfile=${BASE_DIR}log/proxy.pid
stdfile=${BASE_DIR}log/app.log

# sock_file=/var/log/rpc_proxy/rpc_proxy.sock

function check_pid() {
    if [ -f $pidfile ];then
        pid=`cat $pidfile`
        if [ -n $pid ]; then
            running=`ps -p $pid|grep -v "PID TTY" |wc -l`
            return $running
        fi
    fi
    return 0
}

function start() {
    check_pid
    running=$?
    if [ $running -gt 0 ];then
        echo -n "$app now is running already, pid="
        cat $pidfile
        return 1
    fi

    if ! [ -f $conf ];then
        echo "Config file $conf doesn't exist, creating one."
		exit -1
    fi
    
    nohup $app -c $conf -L $logfile --profile-addr=60.29.249.103:7171 &> $stdfile &
    echo $! > $pidfile
    echo "$app started..., pid=$!"
}

function stop() {
    pid=`cat $pidfile`
    kill -15 $pid
    echo "$app stoped..."
}

function restart() {
    stop
    sleep 1
    start
}

function status() {
    check_pid
    running=$?
    if [ $running -gt 0 ];then
        echo started
    else
        echo stoped
    fi
}

function tailf() {
	date=`date +"%Y%m%d"`
    tail -Fn 200 "${logfile}-${date}"
}


function help() {
    echo "$0 start|stop|restart|status|tail"
}

if [ "$1" == "" ]; then
    help
elif [ "$1" == "stop" ];then
    stop
elif [ "$1" == "start" ];then
    start
elif [ "$1" == "restart" ];then
    restart
elif [ "$1" == "status" ];then
    status
elif [ "$1" == "tail" ];then
    tailf
else
    help
fi
