### 结构

```json
{
  "type": "wireguard",
  "tag": "wireguard-out",

  "server": "127.0.0.1",
  "server_port": 1080,
  "system_interface": false,
  "interface_name": "wg0",
  "local_address": [
    "10.0.0.2/32"
  ],
  "private_key": "YNXtAzepDqRv9H52osJVDQnznT5AM11eCK3ESpwSt04=",
  "peer_public_key": "Z1XXLsKYkYxuiYjJIkRvtIKFepCYHTgON+GwPq7SOV4=",
  "pre_shared_key": "31aIhAPwktDGpH4JDhA8GNvjFXEf/a6+UaQRyOAiyfM=",
  "reserved": [0, 0, 0],
  "workers": 4,
  "mtu": 1408,
  "network": "tcp",

  ... // 拨号字段
}
```

!!! warning ""

    默认安装不包含 WireGuard, 参阅 [安装](/zh/#_2)。

!!! warning ""

    默认安装不包含被非特权 WireGuard 需要的 gVisor, 参阅 [安装](/zh/#_2)。

### 字段

#### server

==必填==

服务器地址。

#### server_port

==必填==

服务器端口。

#### system_interface

使用系统 tun 支持。

需要特权且不能与系统接口冲突。

如果 gVisor 未包含在构建中，则强制执行。

#### interface_name

启用 `system_interface` 时的自定义设备名称。

#### local_address

==必填==

接口的 IPv4/IPv6 地址或地址段的列表您。

要分配给接口的 IP（v4 或 v6）地址段列表。

#### private_key

==必填==

WireGuard 需要 base64 编码的公钥和私钥。 这些可以使用 wg(8) 实用程序生成：

```shell
wg genkey
echo "private key" || wg pubkey
```

#### peer_public_key

==必填==

WireGuard 对等公钥。

#### pre_shared_key

WireGuard 预共享密钥。

#### reserved

WireGuard 保留字段字节。

#### workers

WireGuard worker 数量。

默认使用 CPU 数量。

#### mtu

WireGuard MTU。

默认使用 1408。

#### network

启用的网络协议

`tcp` 或 `udp`。

默认所有。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
