!!! quote "sing-box 1.13.0 中的更改"

    :material-plus: [quic_congestion_control](#quic_congestion_control)

### 结构

```json
{
"type": "naive",
"tag": "naive-in",
"network": "udp",

... // 监听字段

"users": [
{
"username": "sekai",
"password": "password"
}
],
"quic_congestion_control": "",
"tls": {}
}
```

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。

### 字段

#### network

监听的网络协议，`tcp` `udp` 之一。

默认所有。

#### users

==必填==

Naive 用户。

#### quic_congestion_control

!!! question "Since sing-box 1.13.0"

QUIC 拥塞控制算法。

| 算法             | 描述                 |
|----------------|--------------------|
| `bbr`          | BBR                |
| `bbr_standard` | BBR (标准版) |
| `bbr2`         | BBRv2              |
| `bbr2_variant` | BBRv2 (一种试验变体)     |
| `cubic`        | CUBIC              |
| `reno`         | New Reno           |

默认使用 `bbr`（NaiveProxy 基于的 Chromium 使用的 QUICHE 的默认值）。

#### tls

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#inbound)。