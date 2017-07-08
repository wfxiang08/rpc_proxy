namespace php RpcThrift.Services

exception RpcException {
  1: i32  code,
  2: string msg
}

service RpcServiceBase {
    void ping();
}