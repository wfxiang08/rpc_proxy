PWD=`pwd`
export GOPATH=$(cd $PWD/..; pwd)
export GOBIN=$GOPATH/bin
echo "----------------"
echo "\$GOPATH = ${GOPATH}"
echo "----------------"