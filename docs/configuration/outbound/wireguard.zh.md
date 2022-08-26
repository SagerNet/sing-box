### 结构

```json
{
  "outbounds": [
    {
      "type": "wireguard",
      "tag": "wireguard-out",
      
      "server": "127.0.0.1",
      "server_port": 1080,
      "local_address": [
        "10.0.0.1",
        "10.0.0.2/32"
      ],
      "private_key": "YNXtAzepDqRv9H52osJVDQnznT5AM11eCK3ESpwSt04=",
      "peer_public_key": "Z1XXLsKYkYxuiYjJIkRvtIKFepCYHTgON+GwPq7SOV4=",
      "pre_shared_key": "31aIhAPwktDGpH4JDhA8GNvjFXEf/a6+UaQRyOAiyfM=",
      "mtu": 1408,
      "network": "tcp",
      
      "detour": "upstream-out",
      "bind_interface": "en0",
      "bind_address": "0.0.0.0",
      "routing_mark": 1234,
      "reuse_addr": false,
      "connect_timeout": "5s",
      "tcp_fast_open": false,
      "domain_strategy": "prefer_ipv6",
      "fallback_delay": "300ms"
    }
  ]
}
```

!!! warning ""

    默认安装不包含 WireGuard, 参阅 [安装](/zh/#_2)。

### WireGuard 字段

#### server

==必填==

服务器地址。

#### server_port

==必填==

服务器端口。

#### local_address

==必填==

接口的 IPv4/IPv6 地址或地址段的列表您。

要分配给接口的 IP（v4 或 v6）地址列表（可以选择带有 CIDR 掩码）。

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

#### mtu

WireGuard MTU。 默认1408。

#### network

启用的网络协议

`tcp` 或 `udp`。

默认所有。

### 拨号字段

#### detour

上游出站的标签。

启用时，其他拨号字段将被忽略。

#### bind_interface

要绑定到的网络接口。

#### bind_address

要绑定的地址。

#### routing_mark

!!! error ""

    仅支持 Linux。

设置 netfilter 路由标记。

#### reuse_addr

重用监听地址。

#### connect_timeout

连接超时，采用 golang 的 Duration 格式。

持续时间字符串是一个可能有符号的序列十进制数，每个都有可选的分数和单位后缀， 例如 "300ms"、"-1.5h" 或 "2h45m"。
有效时间单位为 "ns"、"us"（或 "µs"）、"ms"、"s"、"m"、"h"。

#### domain_strategy

可选值：`prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`。

如果设置，服务器域名将在连接前解析为 IP。

默认使用 `dns.strategy`。

#### fallback_delay

在生成 RFC 6555 快速回退连接之前等待的时间长度。
也就是说，是在假设之前等待 IPv6 成功的时间量如果设置了 "prefer_ipv4"，则 IPv6 配置错误并回退到 IPv4。
如果为零，则使用 300 毫秒的默认延迟。

仅当 `domain_strategy` 为 `prefer_ipv4` 或 `prefer_ipv6` 时生效。
