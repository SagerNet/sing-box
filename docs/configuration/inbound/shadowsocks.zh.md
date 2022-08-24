### 结构

```json
{
  "inbounds": [
    {
      "type": "shadowsocks",
      "tag": "ss-in",
      "listen": "::",
      "listen_port": 5353,
      "tcp_fast_open": false,
      "sniff": false,
      "sniff_override_destination": false,
      "domain_strategy": "prefer_ipv6",
      "udp_timeout": 300,
      "network": "udp",
      "proxy_protocol": false,
      "method": "2022-blake3-aes-128-gcm",
      "password": "8JCsPssfgS8tiRwiMlhARg=="
    }
  ]
}
```

### 多用户结构

```json
{
  "inbounds": [
    {
      "type": "shadowsocks",
      "method": "2022-blake3-aes-128-gcm",
      "password": "8JCsPssfgS8tiRwiMlhARg==",
      "users": [
        {
          "name": "sekai",
          "password": "PCD2Z4o12bKUoFa3cC97Hw=="
        }
      ]
    }
  ]
}
```

### 中转结构

```json
{
  "inbounds": [
    {
      "type": "shadowsocks",
      "method": "2022-blake3-aes-128-gcm",
      "password": "8JCsPssfgS8tiRwiMlhARg==",
      "destinations": [
        {
          "name": "test",
          "server": "example.com",
          "server_port": 8080,
          "password": "PCD2Z4o12bKUoFa3cC97Hw=="
        }
      ]
    }
  ]
}
```

### Shadowsocks 字段

#### network

监听的网络协议，`tcp` `udp` 之一。

默认所有。

#### method

==必填==

| 方法                            | 密钥长度 |
|-------------------------------|------|
| 2022-blake3-aes-128-gcm       | 16   |
| 2022-blake3-aes-256-gcm       | 32   |
| 2022-blake3-chacha20-poly1305 | 32   |
| none                          | /    |
| aes-128-gcm                   | /    |
| aes-192-gcm                   | /    |
| aes-256-gcm                   | /    |
| chacha20-ietf-poly1305        | /    |
| xchacha20-ietf-poly1305       | /    |

#### password

==必填==

| 方法            | 密码格式                          |
|---------------|-------------------------------|
| none          | /                             |
| 2022 methods  | `openssl rand -base64 <密钥长度>` |
| other methods | 任意字符串                         |

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
