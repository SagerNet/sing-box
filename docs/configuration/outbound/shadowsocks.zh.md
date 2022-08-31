### 结构

```json
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

  ... // 拨号字段
}
```

### 字段

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

参阅 [拨号字段](/zh/configuration/shared/dial/)。
