---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

`cloudflared` 入站运行一个内嵌的 Cloudflare Tunnel 客户端，并将所有传入的隧道流量
（TCP、UDP、ICMP）通过 sing-box 的路由引擎转发。

### 结构

```json
{
  "type": "cloudflared",
  "tag": "",

  "token": "",
  "ha_connections": 0,
  "protocol": "",
  "post_quantum": false,
  "edge_ip_version": 0,
  "datagram_version": "",
  "grace_period": "",
  "region": "",
  "control_dialer": {
    ... // 拨号字段
  },
  "tunnel_dialer": {
    ... // 拨号字段
  }
}
```

### 字段

#### token

==必填==

来自 Cloudflare Zero Trust 仪表板的 Base64 编码隧道令牌
（`Networks → Tunnels → Install connector`）。

#### ha_connections

到 Cloudflare edge 的高可用连接数。

上限为已发现的 edge 地址数量。

#### protocol

edge 连接使用的传输协议。

`quic` `http2` 之一。

#### post_quantum

在控制连接上启用后量子密钥交换。

#### edge_ip_version

连接 Cloudflare edge 时使用的 IP 版本。

`0`（自动）`4` `6` 之一。

#### datagram_version

通过 QUIC 进行 UDP 代理时使用的数据报协议版本。

`v2` `v3` 之一。仅在 `protocol` 为 `quic` 时有效。

#### grace_period

正在处理的 edge 连接的优雅关闭窗口。

#### region

Cloudflare edge 区域选择器。

与 `token` 中嵌入的 endpoint 冲突。

#### control_dialer

隧道客户端拨向 Cloudflare 控制面时使用的
[拨号字段](/zh/configuration/shared/dial/)。

#### tunnel_dialer

隧道客户端拨向 Cloudflare edge 数据面时使用的
[拨号字段](/zh/configuration/shared/dial/)。
