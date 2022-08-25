`mixed` 入站是一个 socks4, socks4a, socks5 和 http 服务器.

### 结构

```json
{
  "inbounds": [
    {
      "type": "mixed",
      "tag": "mixed-in",
      
      "listen": "::",
      "listen_port": 2080,
      "tcp_fast_open": false,
      "sniff": false,
      "sniff_override_destination": false,
      "domain_strategy": "prefer_ipv6",
      "proxy_protocol": false,
      
      "users": [
        {
          "username": "admin",
          "password": "admin"
        }
      ],
      "set_system_proxy": false
    }
  ]
}
```

### Mixed 字段

#### users

SOCKS 和 HTTP 用户

默认不需要验证。

#### set_system_proxy

!!! error ""

    仅支持 Linux、Android、Windows 和 macOS。

启动时自动设置系统代理，停止时自动清理。

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
