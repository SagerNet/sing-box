# MASQUE 客户端

!!! question "自 sing-box 1.15.0 起"

`masque-client` endpoint 是一个基于 HTTP 的 IP 代理（[RFC 9484](https://datatracker.ietf.org/doc/html/rfc9484)，CONNECT-IP）客户端。

## 结构

```json
{
  "type": "masque-client",
  "tag": "masque-client",

  "server": "127.0.0.1",
  "server_port": 443,
  "username": "",
  "password": "",
  "path": "",
  "headers": {},
  "version": 0,
  "disable_version_fallback": false,
  "tls": {},
  "advertise_routes": [],
  "system": false,
  "name": "",
  "mtu": 1280,
  "on_demand": false,

  ... // HTTP2 字段 / QUIC 字段
  ... // UDP NAT 字段
  ... // 拨号字段
}
```

!!! note ""

    当内容只有一项时，可以忽略 JSON 数组 [] 标签

## 字段

### server

==必填==

服务器地址。

### server_port

==必填==

服务器端口。

### username

Basic 认证用户名。

### password

Basic 认证密码。

### path

IP 代理资源的 URI 模板路径，可以包含 `target` 和 `ipproto` 变量。

默认使用 `/.well-known/masque/ip/{target}/{ipproto}/`。

### headers

HTTP 请求的额外标头。

### version

HTTP 版本。

可用值：`1`、`2`、`3`。

默认使用 `3`。

当为 `2` 时，[QUIC 字段](#quic-字段) 替换为 [HTTP2 字段](#http2-字段)。

### disable_version_fallback

禁用自动回退到更低的 HTTP 版本。

### tls

TLS 配置，参阅 [TLS](/zh/configuration/shared/tls/#outbound)。

HTTP/3 需要 TLS。

### advertise_routes

向服务器声明的 IP 前缀列表。

服务器会把发往这些前缀的流量路由到此 endpoint，并在此作为入站流量处理。

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

### on_demand

允许该 endpoint 在需要时断开连接。

## HTTP2 字段

当 `version` 为 `2` 时。

参阅 [HTTP2 字段](/zh/configuration/shared/http2/)。

## QUIC 字段

当 `version` 为 `3`（默认）时。

参阅 [QUIC 字段](/zh/configuration/shared/quic/)。

## UDP NAT 字段

参阅 [UDP NAT 字段](/zh/configuration/shared/udp-nat/)。

## 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
