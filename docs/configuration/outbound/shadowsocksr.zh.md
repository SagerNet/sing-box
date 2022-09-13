### 结构

```json
{
  "type": "shadowsocksr",
  "tag": "ssr-out",

  "server": "127.0.0.1",
  "server_port": 1080,
  "method": "aes-128-cfb",
  "password": "8JCsPssfgS8tiRwiMlhARg==",
  "obfs": "plain",
  "obfs_param": "",
  "protocol": "origin",
  "protocol_param": "",
  "network": "udp",

  ... // 拨号字段
}
```

!!! warning ""

    ShadowsocksR 协议已过时且无人维护。 提供此出站仅出于兼容性目的。

!!! warning ""

    默认安装不包含被 ShadowsocksR，参阅 [安装](/zh/#_2)。

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

#### obfs

ShadowsocksR 混淆。

* plain
* http_simple
* http_post
* random_head
* tls1.2_ticket_auth

#### obfs_param

ShadowsocksR 混淆参数。

#### protocol

ShadowsocksR 协议。

* origin
* verify_sha1
* auth_sha1_v4
* auth_aes128_md5
* auth_aes128_sha1
* auth_chain_a
* auth_chain_b

#### protocol_param

ShadowsocksR 协议参数。

#### network

启用的网络协议

`tcp` 或 `udp`。

默认所有。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
