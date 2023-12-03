### 结构

```json
{
  "type": "tuic",
  "tag": "tuic-in",

  ... // 监听字段

  "users": [
    {
      "name": "sekai",
      "uuid": "059032A9-7D40-4A96-9BB1-36823D848068",
      "password": "hello"
    }
  ],
  "congestion_control": "cubic",
  "auth_timeout": "3s",
  "zero_rtt_handshake": false,
  "heartbeat": "10s",
  "tls": {}
}
```

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。

### 字段

#### users

TUIC 用户

#### users.uuid

==必填==

TUIC 用户 UUID

#### users.password

TUIC 用户密码

#### congestion_control

QUIC 拥塞控制算法

可选值: `cubic`, `new_reno`, `bbr`

默认使用 `cubic`。

#### auth_timeout

服务器等待客户端发送认证命令的时间

默认使用 `3s`。

#### zero_rtt_handshake

在客户端启用 0-RTT QUIC 连接握手
这对性能影响不大，因为协议是完全复用的

!!! warning ""
强烈建议禁用此功能，因为它容易受到重放攻击。
请参阅 [Attack of the clones](https://blog.cloudflare.com/even-faster-connection-establishment-with-quic-0-rtt-resumption/#attack-of-the-clones)

#### heartbeat

发送心跳包以保持连接存活的时间间隔

默认使用 `10s`。

#### tls

==必填==

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#inbound)。