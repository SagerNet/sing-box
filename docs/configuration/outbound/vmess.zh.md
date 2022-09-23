### 结构

```json
{
  "type": "vmess",
  "tag": "vmess-out",

  "server": "127.0.0.1",
  "server_port": 1080,
  "uuid": "bf000d23-0752-40b4-affe-68f7707a9661",
  "security": "auto",
  "alter_id": 0,
  "global_padding": false,
  "authenticated_length": true,
  "network": "tcp",
  "tls": {},
  "packet_encoding": "",
  "multiplex": {},
  "transport": {},

  ... // 拨号字段
}
```

### 字段

#### server

==必填==

服务器地址。

#### server_port

==必填==

服务器端口。

#### uuid

==必填==

VMess 用户 ID。

#### security

加密方法：

* `auto`
* `none`
* `zero`
* `aes-128-gcm`
* `chancha20-poly1305`

旧加密方法：

* `aes-128-ctr`

#### alter_id

| Alter ID | 描述         |
|----------|------------|
| 0        | 禁用旧协议      |
| 1        | 启用旧协议      |
| > 1      | 未使用, 行为同 1 |

#### global_padding

协议参数。 如果启用会随机浪费流量（在 v2ray 中默认启用并且无法禁用）。

#### authenticated_length

协议参数。启用长度块加密。

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

#### multiplex

多路复用配置, 参阅 [多路复用](/zh/configuration/shared/multiplex)。

#### transport

V2Ray 传输配置，参阅 [V2Ray 传输层](/zh/configuration/shared/v2ray-transport)。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
