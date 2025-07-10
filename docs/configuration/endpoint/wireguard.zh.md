---
icon: material/new-box
---

!!! question "自 sing-box 1.11.0 起"

### 结构

```json
{
  "type": "wireguard",
  "tag": "wg-ep",

  "system": false,
  "name": "",
  "mtu": 1408,
  "address": [],
  "private_key": "",
  "listen_port": 10000,
  "peers": [
    {
      "address": "127.0.0.1",
      "port": 10001,
      "public_key": "",
      "pre_shared_key": "",
      "allowed_ips": [],
      "persistent_keepalive_interval": 0,
      "reserved": [0, 0, 0]
    }
  ],
  "udp_timeout": "",
  "workers": 0,

  ... // 拨号字段
}
```

!!! note ""

    当内容只有一项时，可以忽略 JSON 数组 [] 标签

### 字段

#### system

使用系统设备。

需要特权且不能与已有系统接口冲突。

#### name

为系统接口自定义设备名称。

#### mtu

WireGuard MTU。

默认使用 1408。

#### address

==必填==

接口的 IPv4/IPv6 地址或地址段的列表。

要分配给接口的 IP（v4 或 v6）地址段列表。

#### private_key

==必填==

WireGuard 需要 base64 编码的公钥和私钥。 这些可以使用 wg(8) 实用程序生成：

```shell
wg genkey
echo "private key" || wg pubkey
```

或 `sing-box generate wg-keypair`.

#### peers

==必填==

WireGuard 对等方的列表。

#### peers.address

对等方的 IP 地址。

#### peers.port

对等方的 WireGuard 端口。

#### peers.public_key

==必填==

对等方的 WireGuard 公钥。

#### peers.pre_shared_key

对等方的预共享密钥。

#### peers.allowed_ips

==必填==

对等方的允许 IP 地址。

#### peers.persistent_keepalive_interval

对等方的持久性保持活动间隔，以秒为单位。

默认禁用。

#### peers.reserved

对等方的保留字段字节。

#### udp_timeout

UDP NAT 过期时间。

默认使用 `5m`。

#### workers

WireGuard worker 数量。

默认使用 CPU 数量。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
