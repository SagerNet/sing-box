---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

### 结构

```json
{
  "type": "trusttunnel",
  "tag": "trusttunnel-out",

  "server": "127.0.0.1",
  "server_port": 443,
  "username": "trust",
  "password": "tunnel",
  "health_check": true,
  "quic": false,
  "quic_congestion_control": "bbr",
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

#### username

==必填==

认证用户名。

#### password

认证密码。

#### health_check

启用周期性健康检查。

#### quic

使用 QUIC 传输。

- `false`：使用基于 TCP 的 HTTP/2。
- `true`：使用基于 UDP 的 HTTP/3。

#### quic_congestion_control

QUIC 拥塞控制算法。

| 算法 | 描述 |
|------|------|
| `bbr` | BBR |
| `bbr_standard` | BBR (标准版) |
| `bbr2` | BBRv2 |
| `bbr_variant` | BBRv2 (一种试验变体) |
| `cubic` | CUBIC |
| `reno` | New Reno |

默认使用 `bbr`。

#### tls

==必填==

出站 TLS 配置，参阅 [TLS](/zh/configuration/shared/tls/#outbound)。

### 拨号字段

拨号字段参阅 [拨号字段](/zh/configuration/shared/dial/)。
