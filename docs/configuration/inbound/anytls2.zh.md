---
icon: material/new-box
---

!!! question "自 sing-box 1.13.0 起"

### 结构

```json
{
  "type": "anytls2",
  "tag": "anytls2-in",

  ... // 监听字段

  "users": [
    {
      "name": "sekai",
      "password": "8JCsPssfgS8tiRwiMlhARg=="
    }
  ],
  "padding_scheme": [],
  "tls": {},
  "transport": {}
}
```

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。

### 字段

#### users

==必填==

AnyTLS 用户。

#### padding_scheme

AnyTLS 填充方案行数组。

默认填充方案:

```json
[
  "stop=8",
  "0=30-30",
  "1=100-400",
  "2=400-500,c,500-1000,c,500-1000,c,500-1000,c,500-1000",
  "3=9-9,500-1000",
  "4=500-1000",
  "5=500-1000",
  "6=500-1000",
  "7=500-1000"
]
```

#### tls

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#入站)。

#### transport

V2Ray 传输配置, 参阅 [V2Ray 传输](/zh/configuration/shared/v2ray-transport/)。

它为 AnyTLS 增加了支持 HTTP 路径的传输能力，使其可以部署在 nginx 一类的反向代理之后，与主站共享同一个公网端口。主站仍然由反向代理直接服务，sing-box 只处理 AnyTLS2 对应的路径。

#### multiplex

不可用。

AnyTLS 已经内置会话多路复用，多个被代理连接会原生共享一个外层连接，不需要也不支持额外的多路复用层。

### 示例（HTTP 路径）

当 nginx 一类反向代理已经监听 `443` 时，AnyTLS2 建议绑定到内部端口（例如 `8443`）。

```json
{
  "type": "anytls2",
  "tag": "anytls2-in",
  "listen": "::",
  "listen_port": 8443,
  "users": [
    {
      "name": "example",
      "password": "password"
    }
  ],
  "transport": {
    "type": "ws",
    "path": "/anytls"
  }
}
```

> **注意：** 当 AnyTLS2 部署在 nginx（或其他反向代理）之后时，无需启用 TLS。代理负责 TLS 终结，并将普通 WebSocket 流量转发到后端。如果直接在公网端口运行 AnyTLS2，则必须在配置中启用 TLS。

#### 最简 nginx 反向代理配置

此示例展示 nginx 终结 TLS 并将 `/anytls` 路径上的 WebSocket 流量转发到 AnyTLS2 后端。AnyTLS2 本身不处理 TLS。

```nginx
server {
    listen 443 ssl;
    server_name example.org;
    ssl_certificate     /path/to/certificate.pem;
    ssl_certificate_key /path/to/key.pem;

    location /anytls {
        proxy_pass http://127.0.0.1:8443;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        # 可选：为长连接调整超时时间
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }
}
```

### 示例（HTTP/2，通过 nginx h2c）

nginx 终结 TLS，通过 h2c（明文 HTTP/2）将流量代理到 AnyTLS2 后端。AnyTLS2 本身不处理 TLS。
HTTP/2 的流多路复用使每个 AnyTLS2 会话对应一个独立的 H2 流，避免了 nginx→后端链路上的 TCP 队头阻塞。

```json
{
  "type": "anytls2",
  "tag": "anytls2-in",
  "listen": "::",
  "listen_port": 8443,
  "users": [
    {
      "name": "example",
      "password": "password"
    }
  ],
  "transport": {
    "type": "http",
    "path": "/anytls-http2"
  }
}
```

对应的 nginx 配置（`proxy_http_version 2` 启用 h2c 上游代理）：

```nginx
server {
    listen 443 ssl;
    server_name example.org;
    ssl_certificate     /path/to/certificate.pem;
    ssl_certificate_key /path/to/key.pem;

    location /anytls-http2 {
        proxy_pass http://127.0.0.1:8443;
        proxy_http_version 2;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }
}
```

### 示例（QUIC，直连）

AnyTLS2 使用 QUIC 传输时直接监听公网 UDP 端口，无需也无法使用反向代理。
QUIC 从根本上消除 TCP 队头阻塞：每个 AnyTLS2 会话对应独立的 QUIC 流，单个流的阻塞不影响其他流。
需要使用 `with_quic` 标签构建的 sing-box。

`quic` 传输不支持路径字段。

```json
{
  "type": "anytls2",
  "tag": "anytls2-in",
  "listen": "::",
  "listen_port": 443,
  "users": [
    {
      "name": "example",
      "password": "password"
    }
  ],
  "tls": {
    "enabled": true,
    "server_name": "example.org",
    "certificate_path": "/path/to/certificate.pem",
    "key_path": "/path/to/key.pem"
  },
  "transport": {
    "type": "quic"
  }
}
```