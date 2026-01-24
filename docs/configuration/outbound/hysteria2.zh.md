!!! quote "sing-box 1.11.0 中的更改"

    :material-plus: [server_ports](#server_ports)  
    :material-plus: [hop_interval](#hop_interval)

### 结构

```json
{
  "type": "hysteria2",
  "tag": "hy2-out",

  "server": "127.0.0.1",
  "server_port": 1080,
  "server_ports": [
    "2080:3000"
  ],
  "hop_interval": "",
  "up_mbps": 100,
  "down_mbps": 100,
  "obfs": {
    "type": "salamander",
    "password": "cry_me_a_r1ver"
  },
  "password": "goofy_ahh_password",
  "network": "tcp",
  "tls": {},
  "brutal_debug": false,
  
  ... // 拨号字段
}
```

!!! note ""

    当内容只有一项时，可以忽略 JSON 数组 [] 标签

!!! warning "与官方 Hysteria2 的区别"

    官方程序支持一种名为 **userpass** 的验证方式，
    本质上上是将用户名与密码的组合 `<username>:<password>` 作为实际上的密码，而 sing-box 不提供此别名。
    要将 sing-box 与官方程序一起使用， 您需要填写该组合作为实际密码。

### 字段

#### server

==必填==

服务器地址。

#### server_port

==必填==

服务器端口。

如果设置了 `server_ports`，则忽略此项。

#### server_ports

!!! question "自 sing-box 1.11.0 起"

服务器端口范围列表。

与 `server_port` 冲突。

#### hop_interval

!!! question "自 sing-box 1.11.0 起"

端口跳跃间隔。

默认使用 `30s`。

#### up_mbps, down_mbps

最大带宽。

如果为空，将使用 BBR 拥塞控制算法而不是 Hysteria CC。

#### obfs.type

QUIC 流量混淆器类型，仅可设为 `salamander`。

如果为空则禁用。

#### obfs.password

QUIC 流量混淆器密码.

#### password

认证密码。

#### network

启用的网络协议。

`tcp` 或 `udp`。

默认所有。

#### tls

==必填==

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#outbound)。

#### brutal_debug

启用 Hysteria Brutal CC 的调试信息日志记录。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
