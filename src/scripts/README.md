# rpc_proxy的系统脚本
`工作目录`: ~/goproject/rpc_proxy/src

## 1. centos 6.5
`旧的机器的部署方式`
* 部署到线上服务器:
	* `bash scripts/deploy_proxy_online.sh hostname`
* 部署到测试服务器:
	* `bash scripts/deploy_proxy_online.sh hostname`
* 只更新lb, proxy
	* `bash scripts/scp_lb.sh  hostname`
	* `bash scripts/scp_proxy.sh  hostname`

## 2. centos 7.0
`ucloud上的部署方式`
* 同时将测试和线上系统服务部署到目标机器:
	* `bash scripts/deploy_proxy_centos7.sh`
		* 将`rpc_proxy, rpc_lb`拷贝到目标机器(包括localhost)的目录下: /usr/local/rpc_proxy/bin/
		* 将`配置文件`拷贝到: /usr/local/rpc_proxy/目录下
			* config.ucloud-online.ini `-->` config.online.ini
			* config.ucloud-test.ini `-->` config.test.ini
        * 拷贝`systemctl脚本`:
	        * rpc_proxy_online.service --> /lib/systemd/system/rpc_proxy_online.service
	        * rpc_proxy_test.service --> /lib/systemd/system/rpc_proxy_test.service
	        * `默认不启动rpc_proxy服务`
	        * `systemctl daemon-reload`
	        * `systemctl enable rpc_proxy_online.service`
	        * systemctl enable rpc_proxy_test.service
		        * enable安装脚本依赖关系，如果enable之后，下次启动时会自动开启
			* systemctl disable rpc_proxy_online.service
			* systemctl disable rpc_proxy_test.service
		        * disable安装脚本依赖关系，如果disable之后，下次启动时不会自动开启
	        * systemctl `start|stop|status|restart` rpc_proxy_test.service
		        * 在enable的情况下，开启动服务
    * rpc_proxy启停的消息，或者异常退出的消息，通过: `/var/log/messages` 来查看
    * 相关的技术:
	    * systemctl
		    * rpc_proxy_online.service
		    * rpc_proxy_test.service
		* 参考:
			* https://coreos.com/docs/launching-containers/launching/getting-started-with-systemd/
