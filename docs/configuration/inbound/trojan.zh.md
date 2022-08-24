### 结构

```json
{
  "inbounds": [
    {
      "type": "trojan",
      "tag": "trojan-in",
      
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
          "password": "8JCsPssfgS8tiRwiMlhARg=="
        }
      ],
      "tls": {},
      "fallback": {
        "server": "127.0.0.0.1",
        "server_port": 8080
      },
      "transport": {}
    }
  ]
}
```

### Trojan 字段

#### users

==必填==

Trojan 用户.

#### tls

==如果启用 HTTP3 则必填==

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#inbound).

#### fallback

!!! error ""

    没有证据表明 GFW 基于 HTTP 响应检测并阻止木马服务器，并且在服务器上打开标准 http/s 端口是一个更大的特征。

备用服务器配置。 如果为空则禁用。

#### transport

V2Ray 传输配置，参阅 [V2Ray 传输层](/zh/configuration/shared/v2ray-transport)。

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

#### proxy_protocol

解析连接头中的 [代理协议](https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt)。
