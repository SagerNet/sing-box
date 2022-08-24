`direct` 入站是一个隧道服务器。

### 结构

```json
{
  "inbounds": [
    {
      "type": "direct",
      "tag": "direct-in",

      "listen": "::",
      "listen_port": 5353,
      "tcp_fast_open": false,
      "sniff": false,
      "sniff_override_destination": false,
      "domain_strategy": "prefer_ipv6",
      "udp_timeout": 300,
      
      "network": "udp",
      "proxy_protocol": false,
      "override_address": "1.0.0.1",
      "override_port": 53
    }
  ]
}
```

### Direct 字段

#### network

监听的网络协议，`tcp` `udp` 之一。

默认所有。

#### override_address

覆盖连接目标地址

#### override_port

覆盖连接目标端口

### 监听字段

#### listen

==必填==

监听地址

#### listen_port

==必填==

监听端口

#### tcp_fast_open

为监听器启用 TCP 快速打开

#### sniff

启用协议探测。

参阅 [协议探测](/zh/configuration/route/sniff/)

#### sniff_override_destination

用探测出的域名覆盖连接目标地址。

如果域名无效（如 Tor），将不生效。

#### domain_strategy

可选值： `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`.

如果设置，请求的域名将在路由之前解析为 IP。

如果 `sniff_override_destination` 生效，它的值将作为后备。

#### udp_timeout

UDP NAT 过期时间，以秒为单位，默认为 300（5 分钟）。

#### proxy_protocol

解析连接头中的 [代理协议](https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt)。
