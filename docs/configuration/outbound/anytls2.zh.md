---
icon: material/new-box
---

!!! question "自 sing-box 1.13.0 起"

### 结构

```json
{
  "type": "anytls2",
  "tag": "anytls2-out",

  "server": "127.0.0.1",
  "server_port": 1080,
  "password": "8JCsPssfgS8tiRwiMlhARg==",
  "idle_session_check_interval": "30s",
  "idle_session_timeout": "30s",
  "min_idle_session": 5,
  "tls": {},
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

#### password

==必填==

AnyTLS 密码。

#### idle_session_check_interval

检查空闲会话的时间间隔。默认值：30秒。

#### idle_session_timeout

在检查中，关闭闲置时间超过此值的会话。默认值：30秒。

#### min_idle_session

在检查中，至少前 `n` 个空闲会话保持打开状态。默认值：`n`=0

#### tls

==必填==

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#出站)。

#### transport

V2Ray 传输配置, 参阅 [V2Ray 传输](/zh/configuration/shared/v2ray-transport/)。

当 AnyTLS 需要 HTTP 路径、WebSocket、gRPC 或 HTTPUpgrade 支持，以便部署在 nginx 一类的反向代理之后并与主站共享同一个公网端口时，使用此字段。

#### multiplex

不可用。

AnyTLS 已经内置会话多路复用，多个被代理连接会原生共享一个外层连接，不需要也不支持额外的多路复用层。

### 示例（HTTP 路径）

```json
{
  "type": "anytls2",
  "tag": "anytls2-out",
  "server": "example.org",
  "server_port": 443,
  "password": "password",
  "tls": {
    "enabled": true,
    "server_name": "example.org"
  },
  "transport": {
    "type": "ws",
    "path": "/anytls"
  }
}
```

### 示例（HTTP/2，通过 nginx h2c）

通过 TLS+HTTP/2 连接到 nginx（端口 443），nginx 再以 h2c 方式代理到 AnyTLS2 后端。
与入站一侧的示例配合使用，参阅对应的 nginx 配置。

```json
{
  "type": "anytls2",
  "tag": "anytls2-out",
  "server": "example.org",
  "server_port": 443,
  "password": "password",
  "tls": {
    "enabled": true,
    "server_name": "example.org"
  },
  "transport": {
    "type": "http",
    "path": "/anytls-http2"
  }
}
```

### 示例（QUIC，直连）

直接通过 QUIC 连接到 AnyTLS2 服务器，无需反向代理。
QUIC 提供独立流交付，彻底消除 TCP 队头阻塞。
需要使用 `with_quic` 标签构建的 sing-box。

`quic` 传输不支持路径字段。

```json
{
  "type": "anytls2",
  "tag": "anytls2-out",
  "server": "example.org",
  "server_port": 443,
  "password": "password",
  "tls": {
    "enabled": true,
    "server_name": "example.org"
  },
  "transport": {
    "type": "quic"
  }
}
```

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。