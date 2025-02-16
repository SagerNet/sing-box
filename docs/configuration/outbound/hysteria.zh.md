---
icon: material/new-box
---

!!! quote "sing-box 1.12.0 中的更改"

    :material-plus: [server_ports](#server_ports)  
    :material-plus: [hop_interval](#hop_interval)

### 结构

```json
{
  "type": "hysteria",
  "tag": "hysteria-out",
  
  "server": "127.0.0.1",
  "server_port": 1080,
  "server_ports": [
    "2080:3000"
  ],
  "hop_interval": "",
  "up": "100 Mbps",
  "up_mbps": 100,
  "down": "100 Mbps",
  "down_mbps": 100,
  "obfs": "fuck me till the daylight",
  "auth": "",
  "auth_str": "password",
  "recv_window_conn": 0,
  "recv_window": 0,
  "disable_mtu_discovery": false,
  "network": "tcp",
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

#### server_ports

!!! question "自 sing-box 1.12.0 起"

服务器端口范围列表。

与 `server_port` 冲突。

#### hop_interval

!!! question "自 sing-box 1.12.0 起"

端口跳跃间隔。

默认使用 `30s`。

#### up, down

==必填==

格式： `[Integer] [Unit]` 例如： `100 Mbps, 640 KBps, 2 Gbps`

支持的单位 (大小写敏感, b = bits, B = bytes, 8b=1B)：

    bps (bits per second)
    Bps (bytes per second)
    Kbps (kilobits per second)
    KBps (kilobytes per second)
    Mbps (megabits per second)
    MBps (megabytes per second)
    Gbps (gigabits per second)
    GBps (gigabytes per second)
    Tbps (terabits per second)
    TBps (terabytes per second)

#### up_mbps, down_mbps

==必填==

以 Mbps 为单位的 `up, down`。

#### obfs

混淆密码。

#### auth

base64 编码的认证密码。

#### auth_str

认证密码。

#### recv_window_conn

用于接收数据的 QUIC 流级流控制窗口。

默认 `15728640 (15 MB/s)`。

#### recv_window

用于接收数据的 QUIC 连接级流控制窗口。

默认 `67108864 (64 MB/s)`。

#### disable_mtu_discovery

禁用路径 MTU 发现 (RFC 8899)。 数据包的大小最多为 1252 (IPv4) / 1232 (IPv6) 字节。

强制为 Linux 和 Windows 以外的系统启用（根据上游）。

#### network

启用的网络协议。

`tcp` 或 `udp`。

默认所有。

#### tls

==必填==

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#outbound)。


### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
