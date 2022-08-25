### Structure

```json
{
  "outbounds": [
    {
      "type": "shadowsocks",
      "tag": "ss-out",
      
      "server": "127.0.0.1",
      "server_port": 1080,
      "method": "2022-blake3-aes-128-gcm",
      "password": "8JCsPssfgS8tiRwiMlhARg==",
      "network": "udp",
      "udp_over_tcp": false,
      "multiplex": {},
      
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

### Shadowsocks 字段

#### server

==必填==

服务器地址。

#### server_port

==必填==

服务器端口。

#### method

==必填==

加密方法：

* `2022-blake3-aes-128-gcm`
* `2022-blake3-aes-256-gcm`
* `2022-blake3-chacha20-poly1305`
* `none`
* `aes-128-gcm`
* `aes-192-gcm`
* `aes-256-gcm`
* `chacha20-ietf-poly1305`
* `xchacha20-ietf-poly1305`

旧加密方法：

* `aes-128-ctr`
* `aes-192-ctr`
* `aes-256-ctr`
* `aes-128-cfb`
* `aes-192-cfb`
* `aes-256-cfb`
* `rc4-md5`
* `chacha20-ietf`
* `xchacha20`

#### password

==必填==

Shadowsocks 密码。

#### network

启用的网络协议

`tcp` 或 `udp`。

默认所有。

#### udp_over_tcp

启用 UDP over TCP 协议。

与 `multiplex` 冲突。

#### multiplex

多路复用配置, 参阅 [多路复用](/zh/configuration/shared/multiplex)。

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
