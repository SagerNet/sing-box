### 结构

```json
{
  "listen": "::",
  "listen_port": 5353,
  "tcp_fast_open": false,
  "tcp_multi_path": false,
  "udp_fragment": false,
  "sniff": false,
  "sniff_override_destination": false,
  "sniff_override_rules": [],
  "sniff_timeout": "300ms",
  "domain_strategy": "prefer_ipv6",
  "udp_timeout": 300,
  "proxy_protocol": false,
  "proxy_protocol_accept_no_header": false,
  "detour": "another-in"
}
```


| 字段                                | 可用上下文                               |
|-----------------------------------|-------------------------------------|
| `listen`                          | 需要监听 TCP 或 UDP。                     |
| `listen_port`                     | 需要监听 TCP 或 UDP。                     |
| `tcp_fast_open`                   | 需要监听 TCP。                           |
| `tcp_multi_path`                  | 需要监听 TCP。                           |
| `udp_timeout`                     | 需要组装 UDP 连接, 当前为 Tun 和 Shadowsocks。 |
| `proxy_protocol`                  | 需要监听 TCP。                           |
| `proxy_protocol_accept_no_header` | `proxy_protocol` 启用时                |

### 字段

#### listen

==必填==

监听地址。

#### listen_port

监听端口。

#### tcp_fast_open

启用 TCP Fast Open。

#### tcp_multi_path

!!! warning ""

    需要 Go 1.21。

启用 TCP Multi Path。

#### udp_fragment

启用 UDP 分段。

#### sniff

启用协议探测。

参阅 [协议探测](/zh/configuration/route/sniff/)

#### sniff_override_destination

用探测出的域名覆盖连接目标地址。

如果域名无效（如 Tor），将不生效。

#### sniff_override_rules

根据规则选择处需要用探测出的域名覆盖目标地址的连接。

参阅 [Sniff Override Rule](/zh/configuration/shared/sniff_override_rules/)

#### sniff_timeout

探测超时时间。

默认使用 300ms。

#### domain_strategy

可选值： `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`。

如果设置，请求的域名将在路由之前解析为 IP。

如果 `sniff_override_destination` 生效，它的值将作为后备。

#### udp_timeout

UDP NAT 过期时间，以秒为单位，默认为 300（5 分钟）。

#### proxy_protocol

解析连接头中的 [代理协议](https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt)。

#### proxy_protocol_accept_no_header

接受没有代理协议标头的连接。

#### detour

如果设置，连接将被转发到指定的入站。

需要目标入站支持，参阅 [注入支持](/zh/configuration/inbound/#_3)。