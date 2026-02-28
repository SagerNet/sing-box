---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

### 结构

```json
{
  "type": "trusttunnel",
  "tag": "trusttunnel-in",

  ... // 监听字段

  "users": [
    {
      "username": "trust",
      "password": "tunnel"
    }
  ],
  "quic_congestion_control": "bbr",
  "network": "tcp,udp",
  "tls": {}
}
```

### 监听字段

监听字段参阅 [监听字段](/zh/configuration/shared/listen/)。

### 字段

#### users

==必填==

TrustTunnel 用户列表。

#### users.username

==必填==

TrustTunnel 用户名。

#### users.password

==必填==

TrustTunnel 用户密码。

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

#### network

网络列表。

可选值：

- `tcp` (HTTP/2)
- `udp` (HTTP/3)

当启用 `udp` 时，必须启用 `tls`。

#### tls

入站 TLS 配置，参阅 [TLS](/zh/configuration/shared/tls/#inbound)。
