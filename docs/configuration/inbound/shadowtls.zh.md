### Structure

```json
{
  "inbounds": [
    {
      "type": "shadowtls",
      "tag": "st-in",
      "listen": "::",
      "listen_port": 443,
      "tcp_fast_open": false,
      "sniff": false,
      "sniff_override_destination": false,
      "domain_strategy": "prefer_ipv6",
      "udp_timeout": 300,
      "proxy_protocol": false,
      
      "handshake": {
        "server": "google.com",
        "server_port": 443,

        "detour": "upstream-out",
        "bind_interface": "en0",
        "bind_address": "0.0.0.0",
        "routing_mark": 1234,
        "reuse_addr": false,
        "connect_timeout": "5s",
        "domain_strategy": "prefer_ipv6",
        "tcp_fast_open": false,
        "fallback_delay": "300ms"
      }
    }
  ]
}
```

### ShadowTLS 字段

#### handshake

==必填==

握手服务器的地址和端口。

##### 拨号字段

###### detour

上游出站的标签。

启用时，其他拨号字段将被忽略。

###### bind_interface

要绑定到的网络接口。

###### bind_address

要绑定的地址。

###### routing_mark

!!! error ""

    仅支持 Linux。

设置 netfilter 路由标记。

###### reuse_addr

重用监听地址。

###### connect_timeout

连接超时，采用 golang 的 Duration 格式。

持续时间字符串是一个可能有符号的序列十进制数，每个都有可选的分数和单位后缀， 例如 "300ms"、"-1.5h" 或 "2h45m"。
有效时间单位为 "ns"、"us"（或 "µs"）、"ms"、"s"、"m"、"h"。

###### domain_strategy

可选值：`prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`。

如果设置，服务器域名将在连接前解析为 IP。

默认使用 `dns.strategy`。

###### fallback_delay

在生成 RFC 6555 快速回退连接之前等待的时间长度。
也就是说，是在假设之前等待 IPv6 成功的时间量如果设置了 "prefer_ipv4"，则 IPv6 配置错误并回退到 IPv4。
如果为零，则使用 300 毫秒的默认延迟。

仅当 `domain_strategy` 为 `prefer_ipv4` 或 `prefer_ipv6` 时生效。

### 监听字段

#### listen

==必填==

监听地址。

#### listen_port

==必填==

监听端口。

#### tcp_fast_open

为监听器启用 TCP 快速打开。

#### sniff

启用协议探测。

参阅 [协议探测](/zh/configuration/route/sniff/)。

#### sniff_override_destination

用探测出的域名覆盖连接目标地址。

如果域名无效（如 Tor），将不生效。

#### domain_strategy

可选值： `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`。

如果设置，请求的域名将在路由之前解析为 IP。

如果 `sniff_override_destination` 生效，它的值将作为后备。

#### proxy_protocol

解析连接头中的 [代理协议](https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt)。