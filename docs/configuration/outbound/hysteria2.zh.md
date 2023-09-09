### 结构

```json
{
  "type": "hysteria2",
  "tag": "hy2-out",

  "server": "127.0.0.1",
  "server_port": 1080,
  "up_mbps": 100,
  "down_mbps": 100,
  "obfs": {
    "type": "salamander",
    "password": "cry_me_a_r1ver"
  },
  "password": "goofy_ahh_password",
  "network": "tcp",
  "tls": {},
  
  ... // 拨号字段
}
```

!!! warning ""

    默认安装不包含被 Hysteria2 依赖的 QUIC，参阅 [安装](/zh/#_2)。

### 字段

#### server

==必填==

服务器地址。

#### server_port

==必填==

服务器端口。

#### up_mbps, down_mbps

最大带宽。

如果为空，将使用 BBR 流量控制算法而不是 Hysteria CC。

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


### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
