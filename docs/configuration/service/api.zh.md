---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

# sing-box API

sing-box API 服务是用于观察与控制正在运行的 sing-box 实例的 gRPC 服务器。

服务器同时接受 [gRPC-Web](https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-WEB.md) 请求,
包括用于双向流方法的 [@improbable-eng/grpc-web](https://github.com/improbable-eng/grpc-web) WebSocket 传输。

### 结构

```json
{
  "type": "api",
  
  ... // 监听字段
  
  "secret": "",
  "access_control_allow_origin": [],
  "access_control_allow_private_network": false,
  "tls": {}
}
```

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。

### 字段

#### secret

API 密钥。

客户端通过标准的 `authorization: Bearer <secret>` gRPC metadata 头认证。

留空则禁用认证。

#### access_control_allow_origin

允许的 CORS 来源,默认使用 `*`。

#### access_control_allow_private_network

允许从私有网络访问。

#### tls

TLS 配置,参阅 [TLS](/zh/configuration/shared/tls/#inbound)。

连接跟踪与 Clash 模式方法需要配置 [Clash API](/zh/configuration/experimental/clash-api/),
否则将以 `UNIMPLEMENTED` 失败。
