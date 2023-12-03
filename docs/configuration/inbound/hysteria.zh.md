### 结构

```json
{
  "type": "hysteria",
  "tag": "hysteria-in",
  
  ... // 监听字段

  "up": "100 Mbps",
  "up_mbps": 100,
  "down": "100 Mbps",
  "down_mbps": 100,
  "obfs": "fuck me till the daylight",

  "users": [
    {
      "name": "sekai",
      "auth": "",
      "auth_str": "password"
    }
  ],

  "recv_window_conn": 0,
  "recv_window_client": 0,
  "max_conn_client": 0,
  "disable_mtu_discovery": false,
  "tls": {}
}
```

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。

### 字段

#### up, down

==必填==

格式: `[Integer] [Unit]` 例如： `100 Mbps, 640 KBps, 2 Gbps`

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

#### users

Hysteria 用户

#### users.auth

base64 编码的认证密码。

#### users.auth_str

认证密码。

#### recv_window_conn

用于接收数据的 QUIC 流级流控制窗口。

默认 `15728640 (15 MB/s)`。

#### recv_window_client

用于接收数据的 QUIC 连接级流控制窗口。

默认 `67108864 (64 MB/s)`。

#### max_conn_client

允许对等点打开的 QUIC 并发双向流的最大数量。

默认 `1024`。

#### disable_mtu_discovery

禁用路径 MTU 发现 (RFC 8899)。 数据包的大小最多为 1252 (IPv4) / 1232 (IPv6) 字节。

强制为 Linux 和 Windows 以外的系统启用（根据上游）。

#### tls

==必填==

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#inbound)。