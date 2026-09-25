# MASQUE 服务器

!!! question "自 sing-box 1.15.0 起"

`masque-server` endpoint 是一个基于 HTTP 的 IP 代理（[RFC 9484](https://datatracker.ietf.org/doc/html/rfc9484)，CONNECT-IP）服务器。

## 结构

```json
{
  "type": "masque-server",
  "tag": "masque-server",

  ... // 监听字段

  "version": [],
  "users": [
    {
      "username": "",
      "password": ""
    }
  ],
  "tls": {},
  "path": "",
  "address": [],
  "advertise_routes": [],
  "system": false,
  "name": "",
  "mtu": 1280,

  ... // HTTP2 字段 / QUIC 字段
  ... // UDP NAT 字段
}
```

!!! note ""

    当内容只有一项时，可以忽略 JSON 数组 [] 标签

## 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。`udp_timeout` 属于下方的 [UDP NAT 字段](#udp-nat-字段)。

## 字段

### version

提供的 HTTP 版本列表。

可用值：`1`、`2`、`3`。

默认提供全部版本。

`3` 需要 TLS。

### users

HTTP 用户，通过 `Authorization` 标头验证。

如果为空则不需要验证。

### tls

TLS 配置，参阅 [TLS](/zh/configuration/shared/tls/#inbound)。

IP 代理必须运行在 TLS 或 QUIC 之上。仅当服务器位于终止 TLS 的 HTTP 中间层之后时才可以不启用。

### path

IP 代理资源的 URI 模板路径，可以包含 `target` 和 `ipproto` 变量。

默认使用 `/.well-known/masque/ip/{target}/{ipproto}/`。

### address

==必填==

隧道网络的 IP 前缀列表，每个 IP 版本最多一个。

前缀中的地址由服务器自己使用，前缀内的其他地址分配给客户端。

### advertise_routes

除隧道网络外，向客户端声明的 IP 前缀列表。

客户端发往其他目的地址的流量会被拒绝。

默认声明全部地址。

### system

使用系统接口。

需要特权且不能与已有系统接口冲突。

如果禁用，sing-box 将使用内部网络栈。

### name

系统接口的自定义接口名称。

默认使用自动生成的 `masque` 接口名称。

### mtu

隧道 MTU。

默认使用 `1280`。

## HTTP2 字段

当 `version` 包含 `2` 时。

参阅 [HTTP2 字段](/zh/configuration/shared/http2/)。

## QUIC 字段

当 `version` 包含 `3`（默认）时，[HTTP2 字段](#http2-字段) 替换为 QUIC 字段。

参阅 [QUIC 字段](/zh/configuration/shared/quic/)。

## UDP NAT 字段

参阅 [UDP NAT 字段](/zh/configuration/shared/udp-nat/)。
