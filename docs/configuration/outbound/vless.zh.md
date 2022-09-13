### 结构

```json
{
  "type": "vless",
  "tag": "vless-out",

  "server": "127.0.0.1",
  "server_port": 1080,
  "uuid": "bf000d23-0752-40b4-affe-68f7707a9661",
  "network": "tcp",
  "tls": {},
  "packet_encoding": "",
  "transport": {},
  
  ... // 拨号字段
}
```

!!! warning ""

    VLESS 协议与 v2ray 架构耦合且无人维护。 提供此出站仅出于兼容性目的。

### 字段

#### server

==必填==

服务器地址。

#### server_port

==必填==

服务器端口。

#### uuid

==必填==

VLESS 用户 ID。

#### network

启用的网络协议。

`tcp` 或 `udp`。

默认所有。

#### tls

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#outbound)。

#### packet_encoding

| 编码         | 描述            |
|------------|---------------|
| (空)        | 禁用            |
| packetaddr | 由 v2ray 5+ 支持 |
| xudp       | 由 xray 支持     |

#### transport

V2Ray 传输配置，参阅 [V2Ray 传输层](/zh/configuration/shared/v2ray-transport)。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
