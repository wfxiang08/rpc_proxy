go build -ldflags "-X main.buildDate=`date +%Y%m%d%H%M%S` -X main.gitVersion=`git -C . rev-parse HEAD`" cmds/rpc_lb.go
