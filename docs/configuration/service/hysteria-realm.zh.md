---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

# Hysteria Realm

Hysteria Realm 是用于 Hysteria2 NAT 穿透的会合服务。

位于 NAT 后面的 Hysteria2 服务器将其通过 STUN 发现的公网地址注册到一个稳定的 realm 端点；客户端从 realm 查询服务器当前的地址并执行 UDP 打洞，以建立直连的 QUIC 连接。

Realm 只承载控制信令。打洞成功后，所有代理流量在客户端和服务器之间直连传输。

### 结构

```json
{
  "type": "hysteria-realm",

  ... // 监听字段

  "tls": {},

  ... // HTTP2 字段

  "users": [
    {
      "name": "",
      "token": "",
      "max_realms": 0
    }
  ]
}
```

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/) 了解详情。

### HTTP2 字段

参阅 [HTTP2 字段](/zh/configuration/shared/http2/) 了解详情。

### 字段

#### tls

TLS 配置，参阅 [TLS](/zh/configuration/shared/tls/#入站)。

配置后，realm 将通过 TLS 提供 HTTP/2 服务；否则提供明文 HTTP/1.1。

#### users

==必填==

授权用户。

#### users.name

==必填==

用户名，用于日志记录和配额键。

#### users.token

==必填==

Hysteria2 入站和出站通过 `Authorization: Bearer <token>` 出示的 Bearer 令牌。

#### users.max_realms

此用户可同时持有的 realm 槽位数量上限。
