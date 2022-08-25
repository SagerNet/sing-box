### 结构

```json
{
  "inbounds": [
    {
      "type": "vmess",
      "tag": "vmess-in",
      
      "listen": "::",
      "listen_port": 2080,
      "tcp_fast_open": false,
      "sniff": false,
      "sniff_override_destination": false,
      "domain_strategy": "prefer_ipv6",
      "proxy_protocol": false,

      "users": [
        {
          "name": "sekai",
          "uuid": "bf000d23-0752-40b4-affe-68f7707a9661",
          "alterId": 0
        }
      ],
      "tls": {},
      "transport": {}
    }
  ]
}
```

### VMess 字段

#### users

==必填==

VMess 用户。

| Alter ID | 描述    |
|----------|-------|
| 0        | 禁用旧协议 |
| > 0      | 启用旧协议 |

!!! warning ""

    提供旧协议支持（VMess MD5 身份验证）仅出于兼容性目的，不建议使用 alterId > 1。

#### tls

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#inbound)。

#### transport

V2Ray 传输配置，参阅 [V2Ray 传输层](/zh/configuration/shared/v2ray-transport)。

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
