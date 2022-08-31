`direct` 出站直接发送请求。

### 结构

```json
{
  "type": "direct",
  "tag": "direct-out",
  
  "override_address": "1.0.0.1",
  "override_port": 53,
  "proxy_protocol": 0,

  ... // 拨号字段
}
```

### 字段

#### override_address

覆盖连接目标地址。

#### override_port

覆盖连接目标端口。

#### proxy_protocol

写出 [代理协议](https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt) 到连接头。

可用协议版本值：`1` 或 `2`。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
