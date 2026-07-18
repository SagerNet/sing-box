# OpenVPN 服务器

!!! question "自 sing-box 1.14.0 起"

## 结构

```json
{
  "type": "openvpn-server",
  "tag": "ovpn-server",

  ... // 监听字段

  "system": false,
  "name": "",
  "mtu": 1500,
  "network": "udp",
  "max_clients": 1024,
  "address": [],
  "topology": "subnet",
  "duplicate_cn": false,
  "users": [
    {
      "username": "",
      "password": ""
    }
  ],
  "tls": {
    "certificate": [],
    "certificate_path": "",
    "key": [],
    "key_path": "",
    "client_certificate": [],
    "client_certificate_path": "",
    "verify_client_certificate": "require",
    "control_wrap": {
      "type": "tls_crypt",
      "key": [],
      "key_path": "",
      "direction": "",
      "force_cookie": false
    }
  },
  "data_ciphers": [],
  "data_ciphers_fallback": "",
  "auth": "",
  "push": {
    "routes": [],
    "dns": [],
    "redirect_gateway": false,
    "redirect_gateway_flags": [],
    "block_outside_dns": false,
    "ping_interval": "",
    "ping_restart": ""
  },
  "ping_interval": "",
  "ping_restart": "",
  "renegotiate_interval": "",
  "handshake_window": "1m",

  ... // UDP NAT 字段
}
```

!!! note ""

    当内容只有一项时，可以忽略 JSON 数组 [] 标签

## 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。`udp_timeout` 属于下方的 [UDP NAT 字段](#udp-nat-字段)。

## 字段

### system

使用系统接口。

需要特权且不能与已有系统接口冲突。

如果禁用，sing-box 将使用内部网络栈。

### name

系统接口的自定义接口名称。

默认使用自动生成的 `ovpn` 接口名称。

### mtu

OpenVPN 接口 MTU。

默认使用 `1500`。

### network

OpenVPN 传输网络，`udp` 或 `tcp` 之一。

默认使用 `udp`。

每个端点仅服务一种传输网络；如需同时服务 TCP 与 UDP，
需要配置两个端点并使用互不重叠的 `address` 子网，
与上游 OpenVPN 需要两个服务进程一致。

### max_clients

已建立与握手中的 TLS 客户端会话的最大数量。

默认使用 `1024`。该值必须小于 OpenVPN peer-id 空间的大小 `16777216`。

### address

==必填==

OpenVPN 服务器地址前缀列表。

最多支持一个 IPv4 前缀和一个 IPv6 前缀。

前缀地址被分配给服务器接口。掩码后的前缀用作客户端地址池和路由。

第一个 IPv4 和 IPv6 前缀地址用作端点的本地地址。

### topology

推送给客户端的 OpenVPN topology，`subnet`、`p2p` 或 `net30` 之一。

默认使用 `subnet`。

### duplicate_cn

允许具有相同认证证书 common name 或用户名的多个客户端同时在线。

禁用时，新认证的会话会替换具有相同身份的现有会话，并在可用时复用其 tunnel 地址。

默认禁用。

### users

OpenVPN 用户名/密码用户列表。

如果设置，客户端除了通过 `tls.verify_client_certificate` 配置的证书策略外，还必须通过用户名/密码认证。

### users.username

用户名。

### users.password

密码。

### tls

==必填==

OpenVPN 控制信道 TLS 配置。

### tls.certificate

TLS 服务器证书内容。

`tls.certificate` 或 `tls.certificate_path` 必填其一。

与 `tls.certificate_path` 冲突。

### tls.certificate_path

TLS 服务器证书路径。

`tls.certificate` 或 `tls.certificate_path` 必填其一。

与 `tls.certificate` 冲突。

### tls.key

TLS 服务器私钥内容。

`tls.key` 或 `tls.key_path` 必填其一。

与 `tls.key_path` 冲突。

### tls.key_path

TLS 服务器私钥路径。

`tls.key` 或 `tls.key_path` 必填其一。

与 `tls.key` 冲突。

### tls.client_certificate

TLS CA 证书内容，用于验证客户端证书。

`tls.client_certificate` 或 `tls.client_certificate_path` 必填其一。

与 `tls.client_certificate_path` 冲突。

### tls.client_certificate_path

TLS CA 证书路径，用于验证客户端证书。

`tls.client_certificate` 或 `tls.client_certificate_path` 必填其一。

与 `tls.client_certificate` 冲突。

### tls.verify_client_certificate

OpenVPN 客户端证书策略，`require`、`optional` 或 `none` 之一。

默认使用 `require`。

设为 `optional` 时，客户端提供证书则验证，不提供证书的客户端也被允许。

设为 `none` 时，不请求客户端证书。

该字段不替代 `users`；设置 `users` 后仍然要求用户名/密码认证。

### tls.control_wrap

OpenVPN 控制信道包装。

等价于 OpenVPN `tls-auth`、`tls-crypt` 和 `tls-crypt-v2`。

默认禁用。

### tls.control_wrap.type

==必填==

控制信道包装类型，`tls_auth`、`tls_crypt` 或 `tls_crypt_v2` 之一。

对于 `tls_crypt_v2`，密钥为服务器密钥。

### tls.control_wrap.key

控制信道包装密钥内容。

`tls.control_wrap.key` 或 `tls.control_wrap.key_path` 必填其一。

与 `tls.control_wrap.key_path` 冲突。

### tls.control_wrap.key_path

控制信道包装密钥路径。

`tls.control_wrap.key` 或 `tls.control_wrap.key_path` 必填其一。

与 `tls.control_wrap.key` 冲突。

### tls.control_wrap.direction

OpenVPN `tls-auth` 密钥方向，`server` 或 `client` 之一。

仅当 `tls.control_wrap.type` 为 `tls_auth` 时可用。

`server` 对应 OpenVPN 密钥方向 `0`，`client` 对应 `1`；按照惯例服务器使用 `0`，客户端使用 `1`。

如果为空，密钥被双向使用，与两端均省略 `key-direction` 的行为一致。

### tls.control_wrap.force_cookie

要求 UDP 上的 `tls-crypt-v2` 客户端支持无状态 session cookie。

仅当 `tls.control_wrap.type` 为 `tls_crypt_v2` 时可用。禁用时，不支持 cookie 的客户端会按照上游 `allow-noncookie` 行为被接受。

默认禁用。

### data_ciphers

允许的 OpenVPN 数据信道加密方式。

默认使用 `AES-256-GCM`、`AES-128-GCM` 和 `CHACHA20-POLY1305`。

### data_ciphers_fallback

用于不支持加密方式协商的遗留客户端的 OpenVPN 数据信道加密方式。

等价于 OpenVPN `data-ciphers-fallback`。

默认禁用。

### auth

OpenVPN 数据信道认证摘要。

默认使用 `SHA1`，与上游默认值一致；仅对非 AEAD 数据信道加密方式和 `tls_auth` 生效。

### push

推送给客户端的选项。

### push.routes

推送给客户端的路由。

IPv4 和 IPv6 前缀可以混用。

### push.dns

推送给客户端的 DNS 服务器地址。

### push.redirect_gateway

向客户端推送 `redirect-gateway`，根据 `push.redirect_gateway_flags` 通过 VPN 路由客户端流量。

当 `push.redirect_gateway_flags` 为空时，默认使用 `def1`。

### push.redirect_gateway_flags

向客户端推送的 OpenVPN `redirect-gateway` flag。

仅当启用 `push.redirect_gateway` 时可用。

默认使用 `def1`。

### push.block_outside_dns

向客户端推送 `block-outside-dns`，在 Windows 客户端上阻止 VPN 之外的 DNS 查询。

### push.ping_interval

推送给客户端的 OpenVPN `ping` 间隔。

在该间隔内未发送任何 packet 后，客户端会向服务器发送一个 data channel ping。

该值必须使用整秒。

默认禁用。

### push.ping_restart

推送给客户端的 OpenVPN `ping-restart` 超时。

在该超时时间内未收到任何 packet 后，客户端会重新连接服务器。

该值必须使用整秒。

默认禁用。

### ping_interval

服务器未向客户端发送任何 packet 时，发送 data channel ping 的间隔。

该值应用于服务器。使用 `push.ping_interval` 配置客户端。

该值必须使用整秒。

默认禁用。

### ping_restart

服务器未收到任何 packet 后关闭客户端会话的时间。

该值应用于服务器。使用 `push.ping_restart` 配置客户端。

服务器超时应长于客户端超时，以便客户端在服务器丢弃其会话前重新连接。

该值必须使用整秒。

默认禁用。

### renegotiate_interval

OpenVPN TLS 重协商间隔。

为空时使用 OpenVPN 默认值 `1h`。

### handshake_window

初始 TLS 握手和每次 TLS 重协商允许使用的最长时间。

默认使用 `1m`。

## UDP NAT 字段

这些字段配置通过 OpenVPN 接口的流量的 UDP 会话。

参阅 [UDP NAT 字段](/zh/configuration/shared/udp-nat/)。
