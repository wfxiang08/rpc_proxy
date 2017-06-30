#!/bin/bash

# control_lb脚本必须和具体的产品项目放在一起
WORKSPACE=$(cd $(dirname $0)/; pwd)
cd $WORKSPACE
# 确保log目录存在(相对产品放在一起)
mkdir -p log
conf=config.ini
logfile=log/lb.log
pidfile=log/app_lb.pid
stdfile=log/app_lb.log


BASE_DIR=/usr/local/rpc_proxy/
app=${BASE_DIR}bin/rpc_lb


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
	
	GIT_URL=`git config --get remote.origin.url`
	GIT_VERSION=`git rev-parse --short HEAD`
	
	# 每次重启之后 stdfile被覆盖
    nohup $app -c $conf -L $logfile --work-dir=${WORKSPACE} --code-url-version="${GIT_URL}@${GIT_VERSION}" &> $stdfile &
    echo $! > $pidfile
    echo "$app started..., pid=$!"
}

function stop() {
	check_pid
	running=$?
	# 由于lb等需要graceful stop, 因此stop过程需要等待
	if [ $running -gt 0 ];then
	    pid=`cat $pidfile`
		kill -15 $pid
		status="0"
		while [ "$status" == "0" ];do
			echo "Waiting for process ${pid} ..."
			sleep 1
			ps -p$pid 2>&1 > /dev/null
			status=$?
		done
	    echo "$app stoped..."
	else
		echo "$app already stoped..."
	fi
}

function restart() {
    stop
    sleep 1
    start
}

# 查看当前的进程的状态
function status() {
    check_pid
    running=$?
    if [ $running -gt 0 ];then
        echo started
    else
        echo stoped
    fi
}

# 查看最新的Log文件
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
