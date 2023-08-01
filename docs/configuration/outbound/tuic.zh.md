### 结构

```json
{
  "type": "tuic",
  "tag": "tuic-out",
  
  "server": "127.0.0.1",
  "server_port": 1080,
  "uuid": "2DD61D93-75D8-4DA4-AC0E-6AECE7EAC365",
  "password": "hello",
  "congestion_control": "cubic",
  "udp_relay_mode": "native",
  "zero_rtt_handshake": false,
  "heartbeat": "10s",
  "network": "tcp",
  "tls": {},
  
  ... // 拨号字段
}
```

!!! warning ""

    默认安装不包含被 TUI 依赖的 QUIC，参阅 [安装](/zh/#_2)。

### 字段

#### server

==必填==

服务器地址。

#### server_port

==必填==

服务器端口。

#### uuid

==必填==

TUIC 用户 UUID

#### password

TUIC 用户密码

#### congestion_control

QUIC 流量控制算法

可选值: `cubic`, `new_reno`, `bbr`

默认使用 `cubic`。

#### udp_relay_mode

UDP 包中继模式

| 模式     | 描述                           |
|--------|------------------------------|
| native | 原生 UDP                       |
| quic   | 使用 QUIC 流的无损 UDP 中继，引入了额外的开销 |


#### zero_rtt_handshake

在客户端启用 0-RTT QUIC 连接握手
这对性能影响不大，因为协议是完全复用的

!!! warning ""
强烈建议禁用此功能，因为它容易受到重放攻击。
请参阅 [Attack of the clones](https://blog.cloudflare.com/even-faster-connection-establishment-with-quic-0-rtt-resumption/#attack-of-the-clones)

#### heartbeat

发送心跳包以保持连接存活的时间间隔

#### network

启用的网络协议。

`tcp` 或 `udp`。

默认所有。

#### tls

==必填==

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#outbound)。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
