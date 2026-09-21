`http` 出站是一个 HTTP CONNECT 代理客户端

### 结构

```json
{
  "type": "http",
  "tag": "http-out",
  
  "server": "127.0.0.1",
  "server_port": 1080,
  "username": "sekai",
  "password": "admin",
  "path": "",
  "headers": {},
  "version": 0,
  "disable_version_fallback": false,
  "tls": {},

  ... // HTTP2 字段 / QUIC 字段
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

#### username

Basic 认证用户名。

#### password

Basic 认证密码。

#### path

HTTP 请求路径。

#### headers

HTTP 请求的额外标头。

#### version

!!! question "自 sing-box 1.15.0 起"

HTTP 版本。

可用值：`1`、`2`、`3`。

默认使用 `2`；设置了 `path` 或 `Host` 头时默认使用 `1`。

`path` 和 `Host` 头仅在 `1` 时可用。

当为 `3` 时，[HTTP2 字段](#http2-字段) 替换为 [QUIC 字段](#quic-字段)。

#### disable_version_fallback

!!! question "自 sing-box 1.15.0 起"

禁用自动回退到更低的 HTTP 版本。

#### tls

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#出站)。

### HTTP2 字段

!!! question "自 sing-box 1.15.0 起"

当 `version` 为 `2`（默认）时。

参阅 [HTTP2 字段](/zh/configuration/shared/http2/) 了解详情。

### QUIC 字段

!!! question "自 sing-box 1.15.0 起"

当 `version` 为 `3` 时。

参阅 [QUIC 字段](/zh/configuration/shared/quic/) 了解详情。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
