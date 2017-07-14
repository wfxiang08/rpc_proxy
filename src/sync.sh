rsync -avp --exclude=.git --exclude=vendor --exclude=.idea --exclude='/tool_*'  --exclude='/task_*' --exclude='/service_*'  . media1:/root/workspace/rpc_proxy/src
