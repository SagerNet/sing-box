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
  "udp_over_stream": false,
  "zero_rtt_handshake": false,
  "heartbeat": "10s",
  "network": "tcp",
  "tls": {},
  
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

TUIC 用户 UUID

#### password

TUIC 用户密码

#### congestion_control

QUIC 拥塞控制算法

可选值: `cubic`, `new_reno`, `bbr`

默认使用 `cubic`。

#### udp_relay_mode

UDP 包中继模式

| 模式     | 描述                           |
|--------|------------------------------|
| native | 原生 UDP                       |
| quic   | 使用 QUIC 流的无损 UDP 中继，引入了额外的开销 |

与 `udp_over_stream` 冲突。

#### udp_over_stream

这是 TUIC 的 [UDP over TCP 协议](/configuration/shared/udp-over-tcp/) 移植， 旨在提供 TUIC 不提供的 基于 QUIC 流的 UDP 中继模式。 由于它是一个附加协议，因此您需要使用 sing-box 或其他兼容的程序作为服务器。

此模式在正确的 UDP 代理场景中没有任何积极作用，仅适用于中继流式 UDP 流量（基本上是 QUIC 流）。

与 `udp_relay_mode` 冲突。

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
