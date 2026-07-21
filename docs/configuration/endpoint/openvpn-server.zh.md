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
  "mode": "tls",
  "network": "udp",
  "remote": "",
  "remote_port": 0,
  "max_clients": 1024,
  "address": [],
  "peer_address": "",
  "peer_address_ipv6": "",
  "topology": "subnet",
  "duplicate_cn": false,
  "users": [
    {
      "username": "",
      "password": ""
    }
  ],
  "static_key": [],
  "static_key_path": "",
  "key_direction": "",
  "tls": {
    "certificate": [],
    "certificate_path": "",
    "key": [],
    "key_path": "",
    "client_certificate": [],
    "client_certificate_path": "",
    "verify_client_certificate": "require",
    "client_name": "",
    "client_name_type": "name",
    "peer_fingerprint": [],
    "crl_path": "",
    "remote_certificate_ku": [],
    "remote_certificate_eku": "",
    "remote_certificate_tls": "",
    "certificate_profile": "",
    "ns_certificate_type": "",
    "version_min": "1.2",
    "version_max": "",
    "cipher": "",
    "groups": "",
    "control_wrap": {
      "type": "tls_crypt",
      "key": [],
      "key_path": "",
      "direction": "",
      "force_cookie": false
    }
  },
  "cipher": "",
  "data_ciphers": [],
  "data_ciphers_fallback": "",
  "auth": "",
  "mss_fix": 0,
  "mss_fix_disabled": false,
  "mss_fix_mode": "",
  "replay_window": 0,
  "replay_window_time": "",
  "push": {
    "routes": [],
    "dns": [],
    "dns_servers": [],
    "search_domains": [],
    "dhcp_options": [],
    "redirect_gateway": false,
    "redirect_gateway_flags": [],
    "block_outside_dns": false,
    "ping_interval": "",
    "ping_restart": ""
  },
  "ping_interval": "",
  "ping_restart": "",
  "renegotiate_interval": "",
  "renegotiate_disabled": false,
  "renegotiate_bytes": 0,
  "renegotiate_packets": 0,
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

endpoint 会配置接口地址和 MTU，但不会安装操作系统路由或 DNS 设置。

如果禁用，sing-box 将使用内部网络栈。

### name

系统接口的自定义接口名称。

默认使用自动生成的 `ovpn` 接口名称。

### mtu

OpenVPN 接口 MTU。

默认使用 `1500`。

### mode

OpenVPN 会话模式，`tls` 或 `static_key` 之一。

默认使用 `tls`。

`static_key` 在没有 TLS 控制信道和前向保密的情况下服务一个对端，仅作为不可变部署的显式兼容选项保留。该模式不使用 `tls`、`users`、推送选项或 TLS 重协商选项。

### network

OpenVPN 传输网络，`udp` 或 `tcp` 之一。

默认使用 `udp`。

每个端点仅服务一种传输网络；如需同时服务 TCP 与 UDP，
需要配置两个端点并使用互不重叠的 `address` 子网，
与上游 OpenVPN 需要两个服务进程一致。

### remote

UDP `static_key` 服务器的固定远端地址。

在 UDP `static_key` 模式下与 `remote_port` 一起必填。TCP 服务器从监听套接字接受单个对端，不使用此字段。

### remote_port

UDP `static_key` 服务器的固定远端端口。

在 UDP `static_key` 模式下与 `remote` 一起必填。

### max_clients

已建立与握手中的 TLS 客户端会话的最大数量。

默认使用 `1024`。该值必须小于 OpenVPN peer-id 空间的大小 `16777216`。

`static_key` 模式仅支持一个对端，因此此值必须为 `0` 或 `1`。

### address

==必填==

OpenVPN 服务器地址前缀列表。

最多支持一个 IPv4 前缀和一个 IPv6 前缀。

前缀地址被分配给服务器接口。掩码后的前缀用作客户端地址池和路由。

第一个 IPv4 和 IPv6 前缀地址用作端点的本地地址。

在 `static_key` 模式下，这些地址是本地隧道前缀，而不是地址池。

### peer_address

IPv4 隧道对端地址。

在 `static_key` 模式下配置 IPv4 `address` 时必填。

### peer_address_ipv6

IPv6 隧道对端地址。

在 `static_key` 模式下配置 IPv6 `address` 时必填。

### topology

推送给客户端的 OpenVPN topology，`subnet`、`p2p` 或 `net30` 之一。

TLS 模式默认使用 `subnet`，`static_key` 模式默认使用 `p2p`。

### duplicate_cn

允许具有相同认证证书 common name 或用户名的多个客户端同时在线。

禁用时，新认证的会话会替换具有相同身份的现有会话，并在可用时复用其 tunnel 地址。

默认禁用。

仅在 TLS 模式下可用。

### users

OpenVPN 用户名/密码用户列表。

如果设置，客户端除了通过 `tls.verify_client_certificate` 配置的证书策略外，还必须通过用户名/密码认证。

仅在 TLS 模式下可用。

### users.username

用户名。

### users.password

密码。

### static_key

OpenVPN 静态密钥内容。

在 `static_key` 模式下必填。

与 `static_key_path` 冲突。

### static_key_path

OpenVPN 静态密钥路径。

在 `static_key` 模式下未设置 `static_key` 时必填。

与 `static_key` 冲突。

### key_direction

静态密钥方向，`server` 或 `client` 之一。

为空时双向使用密钥。按照惯例，服务器使用 `server`，对端使用 `client`。

仅在 `static_key` 模式下可用。

### tls

在 TLS 模式下必填。

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

当 `tls.verify_client_certificate` 为 `require` 或 `optional` 时，`tls.client_certificate`、`tls.client_certificate_path` 或 `tls.peer_fingerprint` 必填其一。

与 `tls.client_certificate_path` 冲突。

### tls.client_certificate_path

TLS CA 证书路径，用于验证客户端证书。

当 `tls.verify_client_certificate` 为 `require` 或 `optional` 时，`tls.client_certificate`、`tls.client_certificate_path` 或 `tls.peer_fingerprint` 必填其一。

与 `tls.client_certificate` 冲突。

### tls.verify_client_certificate

OpenVPN 客户端证书策略，`require`、`optional` 或 `none` 之一。

默认使用 `require`。

设为 `optional` 时，客户端提供证书则验证，不提供证书的客户端也被允许。

设为 `none` 时，不请求客户端证书。

该字段不替代 `users`；设置 `users` 后仍然要求用户名/密码认证。

### tls.client_name

期望的客户端证书名称。为空时禁用。

### tls.client_name_type

`tls.client_name` 匹配的证书字段，`subject`、`name` 或 `name-prefix` 之一。

配置 `tls.client_name` 时默认使用 `name`。

### tls.peer_fingerprint

允许的客户端叶证书 SHA-256 指纹。可以在没有客户端 CA 时仅使用指纹验证。

### tls.crl_path

用于拒绝已吊销客户端证书的证书吊销列表路径。

### tls.remote_certificate_ku

OpenVPN `remote-cert-ku` 格式的客户端证书 Key Usage mask。

### tls.remote_certificate_eku

客户端证书所需的 Extended Key Usage。与显式配置的 `tls.remote_certificate_tls` 冲突。

### tls.remote_certificate_tls

客户端证书用途检查，`server`、`client` 或 `none` 之一。默认使用 `client`。

### tls.certificate_profile

证书 profile，可选值为 `insecure`、`legacy`、`preferred` 或 `suiteb`。

默认使用 `legacy`。

`insecure` 为兼容不可变对端而接受使用 MD5 或 SHA-1 签名的证书链和较小的旧密钥，仅应在对端无法升级时使用。`legacy` 接受 SHA-1 但拒绝 MD5 签名；`preferred` 要求更强的签名和密钥。

选择 `suiteb` 且 `tls.cipher` 为空时，TLS 1.2 cipher 列表默认使用 Suite B ECDHE-ECDSA AES-GCM 套件。该 profile 不限制显式配置的 `tls.cipher` 和 `tls.groups`。

### tls.ns_certificate_type

已弃用的 Netscape 证书类型检查，`server` 或 `client` 之一。

### tls.version_min

最低 TLS 版本。默认使用 `1.2`。

### tls.version_max

最高 TLS 版本。默认使用支持的最高版本。

### tls.cipher

TLS 1.2 及更低版本允许的 OpenSSL cipher suite 名称，以冒号分隔。

为空时使用默认 TLS cipher suite。该字段不控制 TLS 1.3 cipher suite。

### tls.groups

按偏好顺序排列的 TLS key exchange group，以冒号分隔。

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

### cipher

`static_key` 模式使用的数据信道加密方式。

为空时使用上游静态密钥模式的默认值 `BF-CBC`。支持 AES-CBC、ARIA-CBC、Camellia-CBC、DES-CBC、Blowfish-CBC、CAST5-CBC 系列，以及 `SEED-CBC`、`SM4-CBC` 和 `NONE`。

仅在 `static_key` 模式下可用。`NONE` 不提供机密性。

### data_ciphers

允许的 OpenVPN 数据信道加密方式。

默认使用 `AES-256-GCM`、`AES-128-GCM` 和 `CHACHA20-POLY1305`。

AES-GCM 系列还包括 `AES-192-GCM`。保留的 cipher 包括 AES、ARIA、Camellia、DES、Blowfish、CAST5、SEED 和 SM4 的 CBC、CFB、OFB 形式，以及 `NONE`。CFB 和 OFB 仅可用于 TLS 模式。旧 cipher 只能提供较弱的机密性或完全不加密，因此默认不启用。

仅在 TLS 模式下可用。

### data_ciphers_fallback

用于不支持加密方式协商的遗留客户端的 OpenVPN 数据信道加密方式。

等价于 OpenVPN `data-ciphers-fallback`。

默认禁用。

仅在 TLS 模式下可用。

### auth

OpenVPN 数据信道认证摘要。

默认使用 `SHA1`，与上游默认值一致；仅对非 AEAD 数据信道加密方式和 `tls_auth` 生效。

为兼容既有客户端，显式配置时仍可使用 `MD5` 和 `RIPEMD160` 等旧摘要。

### mss_fix

用于限制 TCP MSS 的最大封装数据包大小。默认 MTU 下使用上游默认值 `1492` 计算。

### mss_fix_disabled

禁用 MSS 限制，包括默认限制。

### mss_fix_mode

显式 `mss_fix` 的计算模式，`mtu` 或 `fixed` 之一。需要 `mss_fix`。

### replay_window

UDP 数据通道重放窗口大小。默认使用 `64`；TCP 数据包 ID 始终严格连续。

### replay_window_time

UDP 重放窗口时长。默认使用 `15s`。该值必须使用整秒。

### push

推送给客户端的选项。

### push.routes

推送给客户端的路由。

IPv4 和 IPv6 前缀可以混用。

### push.dns

推送给客户端的 DNS 服务器地址。

使用传统的 `dhcp-option DNS`/`DNS6`。兼容客户端收到现代 DNS 服务器组时会覆盖这些地址。

### push.dns_servers

推送的现代 OpenVPN DNS 服务器组。每项包含 `priority`、`addresses`，以及可选的 `resolve_domains`、`dnssec`、`transport` 和 `sni`。

地址接受 IP 或 `IP:port`（带端口的 IPv6 使用 `[IPv6]:port`）。`transport` 为 `plain`、`dot` 或 `doh` 之一；`dnssec` 为 `yes`、`optional` 或 `no` 之一。OpenVPN 客户端仅应用优先级数字最低的服务器组。

### push.search_domains

推送的现代 OpenVPN 搜索域。

### push.dhcp_options

推送的额外传统 `dhcp-option` 值，不包含 `dhcp-option` 前缀。

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

仅在 TLS 模式下可用。

### renegotiate_disabled

禁用基于时间的 TLS 重协商，包括默认间隔。

仅在 TLS 模式下可用。

### renegotiate_bytes

传输指定字节数后重新协商数据通道密钥。`0` 使用与密码算法相关的 OpenVPN 默认值。

仅在 TLS 模式下可用。

### renegotiate_packets

传输指定数据包数后重新协商数据通道密钥。`0` 使用与密码算法相关的 OpenVPN 默认值。

仅在 TLS 模式下可用。

### handshake_window

初始 TLS 握手和每次 TLS 重协商允许使用的最长时间。

默认使用 `1m`。

仅在 TLS 模式下可用。

## UDP NAT 字段

这些字段配置通过 OpenVPN 接口的流量的 UDP 会话。

参阅 [UDP NAT 字段](/zh/configuration/shared/udp-nat/)。
