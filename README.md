# RPC Proxy IpLocation
* go build cmds/service_iplocations.go
* ./service_iplocations -c config.ini
* go build gen-go/ip_service/ip_service-remote/ip_service-remote.go
* ./ip_service-remote -h 127.0.0.1 -p 5563 -framed IpToLocation "120.52.139.4"
Location({City:北京市 Province:北京市 Detail:联通云BGP数据中心}) <nil>