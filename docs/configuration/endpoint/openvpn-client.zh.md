# OpenVPN 客户端

!!! question "自 sing-box 1.14.0 起"

## 结构

```json
{
  "type": "openvpn-client",
  "tag": "ovpn-client",

  "mode": "tls",
  "server": "127.0.0.1",
  "server_port": 1194,
  "servers": [
    {
      "server": "127.0.0.1",
      "server_port": 1194,
      "network": "udp"
    }
  ],
  "remote_random": false,
  "network": "udp",
  "address": [],
  "peer_address": "",
  "peer_address_ipv6": "",
  "topology": "",
  "username": "",
  "password": "",
  "auth_retry": "none",
  "static_challenge": "",
  "static_challenge_echo": false,
  "static_key": [],
  "static_key_path": "",
  "key_direction": "",
  "tls": {
    "server_name": "",
    "server_name_type": "name",
    "certificate": [],
    "certificate_path": "",
    "client_certificate": [],
    "client_certificate_path": "",
    "client_key": [],
    "client_key_path": "",
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
      "type": "",
      "key": [],
      "key_path": "",
      "direction": ""
    }
  },
  "cipher": "",
  "data_ciphers": [],
  "data_ciphers_fallback": "",
  "auth": "",
  "mss_fix": 0,
  "mss_fix_disabled": false,
  "mss_fix_mode": "",
  "fragment": 0,
  "replay_window": 0,
  "replay_window_time": "",
  "compression": "",
  "compression_lzo": "",
  "allow_compression": "no",
  "route_no_pull": false,
  "pull_filters": [
    {
      "action": "ignore",
      "text": "route "
    }
  ],
  "routes": [],
  "route_gateway": "",
  "route_metric": 0,
  "redirect_gateway": false,
  "redirect_gateway_flags": [],
  "redirect_private": false,
  "block_ipv6": false,
  "ping_interval": "",
  "ping_restart": "",
  "ping_restart_disabled": false,
  "renegotiate_interval": "",
  "renegotiate_disabled": false,
  "renegotiate_bytes": 0,
  "renegotiate_packets": 0,
  "tls_timeout": "",
  "handshake_window": "",
  "explicit_exit_notify": 0,
  "system": false,
  "name": "",
  "mtu": 1500,

  ... // UDP NAT 字段

  ... // 拨号字段
}
```

!!! note ""

    当内容只有一项时，可以忽略 JSON 数组 [] 标签。

## 字段

### mode

OpenVPN 会话模式，可选值为 `tls` 或 `static_key`。

默认使用 `tls`。

`static_key` 是已弃用的 OpenVPN 模式，不使用 TLS 控制通道且不提供前向保密。
为兼容无法修改的企业 VPN 服务器，此模式仍作为显式兼容选项保留。该模式不使用
`tls`、用户名/密码认证、拉取选项或 TLS 重协商选项。

### server

OpenVPN 服务器地址。

`server` 和 `servers` 之一必填。

与 `servers` 冲突。

### server_port

OpenVPN 服务器端口。

设置 `server` 时必填。

### servers

OpenVPN 服务器列表。

客户端按顺序尝试服务器，并在连接失败时尝试下一台服务器。

`server` 和 `servers` 之一必填。

与 `server` 冲突。

### servers.server

==必填==

OpenVPN 服务器地址。

### servers.server_port

==必填==

OpenVPN 服务器端口。

### servers.network

该服务器的 OpenVPN 传输网络，可选值为 `udp` 或 `tcp`。

默认使用顶层 `network`。

### remote_random

连接前随机排列 `servers` 顺序。

默认禁用。

### network

默认 OpenVPN 传输网络，可选值为 `udp` 或 `tcp`。

默认使用 `udp`。

该值应用于 `server` 和未单独设置 `network` 的 `servers` 条目。

### address

本地 IPv4 和 IPv6 隧道前缀。

`static_key` 模式至少需要一个地址。在 TLS 模式下该字段可选，并可被服务器推送的地址替换。

### peer_address

IPv4 隧道对端地址及 VPN 网关。

在 `static_key` 模式下配置 IPv4 `address` 时必填。

### peer_address_ipv6

IPv6 隧道对端地址及 VPN 网关。

在 `static_key` 模式下配置 IPv6 `address` 时必填。

### topology

隧道拓扑，可选值为 `net30`、`p2p` 或 `subnet`。

TLS 模式下为空时使用服务器推送的拓扑。

### username

OpenVPN 用户名/密码认证的用户名。

仅在 TLS 模式下可用。

### password

OpenVPN 用户名/密码认证的密码。

### auth_retry

用户名/密码认证失败后的行为，可选值为 `none`、`nointeract` 或 `interact`。

默认使用 `none`，并将永久认证失败视为终止错误。

`nointeract` 和 `interact` 允许重试认证。

### static_challenge

请求认证响应时显示的静态质询文本。

### static_challenge_echo

以明文显示静态质询响应。

### static_key

OpenVPN 静态密钥内容。

在 `static_key` 模式下必填。

与 `static_key_path` 冲突。

### static_key_path

OpenVPN 静态密钥路径。

在 `static_key` 模式下未设置 `static_key` 时必填。

与 `static_key` 冲突。

### key_direction

静态密钥方向，可选值为 `server` 或 `client`。

为空时双向使用密钥。仅在 `static_key` 模式下可用。

### tls

在 TLS 模式下必填。

OpenVPN 控制通道 TLS 配置。

### tls.server_name

预期的服务器证书名称。

为空时禁用证书名称验证，但仍会验证证书链或 fingerprint 与服务器证书用途。

### tls.server_name_type

与 `tls.server_name` 匹配的证书字段，可选值为 `subject`、`name` 或 `name-prefix`。

设置 `tls.server_name` 时默认使用 `name`。

`subject` 匹配完整证书 subject，`name` 精确匹配 common name，`name-prefix` 匹配 common name 前缀。

### tls.certificate

受信任 CA 证书内容。

`tls.certificate`、`tls.certificate_path` 和 `tls.peer_fingerprint` 之一必填。

与 `tls.certificate_path` 冲突。

### tls.certificate_path

受信任 CA 证书路径。

`tls.certificate`、`tls.certificate_path` 和 `tls.peer_fingerprint` 之一必填。

与 `tls.certificate` 冲突。

### tls.client_certificate

客户端证书内容。

与 `tls.client_certificate_path` 冲突。

### tls.client_certificate_path

客户端证书路径。

与 `tls.client_certificate` 冲突。

### tls.client_key

客户端私钥内容。

与 `tls.client_key_path` 冲突。

### tls.client_key_path

客户端私钥路径。

与 `tls.client_key` 冲突。

客户端证书和私钥必须同时设置或同时为空。

### tls.peer_fingerprint

允许的服务器 leaf certificate 的 SHA-256 fingerprint。

每个 fingerprint 必须是不带分隔符的 64 字符小写十六进制字符串。

同时配置受信任 CA 时，会同时验证证书链和 fingerprint。未配置受信任 CA 时，会验证 fingerprint、证书有效期、配置的名称和证书用途，但不验证证书链。

### tls.crl_path

用于拒绝已吊销服务器证书的 PEM 或 DER CRL 文件路径。

根据受信任证书链验证 CRL 签名和有效期。

默认禁用。

### tls.remote_certificate_ku

服务器证书所需的 Key Usage mask，使用 OpenVPN `remote-cert-ku` 格式的十六进制值。

证书必须包含至少一个已配置 mask 中的所有 bit。

默认禁用。

### tls.remote_certificate_eku

服务器证书所需的 Extended Key Usage。

接受 OpenSSL 名称、Object Identifier 以及 `server` 和 `client` 别名。

设置后，该字段会替代默认的 `tls.remote_certificate_tls` 检查。

与显式配置的 `tls.remote_certificate_tls` 冲突。

### tls.remote_certificate_tls

对端证书用途检查，可选值为 `server`、`client` 或 `none`。

默认使用 `server`。

`none` 禁用证书用途检查。

与 `tls.remote_certificate_eku` 冲突。

### tls.certificate_profile

证书 profile，可选值为 `insecure`、`legacy`、`preferred` 或 `suiteb`。

默认使用 `legacy`。

`insecure` 为兼容不可变对端而接受使用 MD5 或 SHA-1 签名的证书链和较小的旧密钥，仅应在对端无法升级时使用。`legacy` 接受 SHA-1 但拒绝 MD5 签名；`preferred` 要求更强的签名和密钥。

选择 `suiteb` 且 `tls.cipher` 为空时，TLS 1.2 cipher 列表默认使用 Suite B ECDHE-ECDSA AES-GCM 套件。该 profile 不限制显式配置的 `tls.cipher` 和 `tls.groups`。

### tls.ns_certificate_type

已弃用的 Netscape 证书类型检查，`server` 或 `client` 之一。

默认禁用。请优先使用 `tls.remote_certificate_tls`。

### tls.version_min

最低 TLS 版本，可选值为 `1.0`、`1.1`、`1.2` 或 `1.3`。

默认使用 `1.2`。

### tls.version_max

最高 TLS 版本，可选值为 `1.0`、`1.1`、`1.2` 或 `1.3`。

默认使用支持的最高版本。

该值不能低于 `tls.version_min`。

### tls.cipher

TLS 1.2 及更低版本允许的 OpenSSL cipher suite 名称，以冒号分隔。

为空时使用默认 TLS cipher suite。该字段不控制 TLS 1.3 cipher suite。

### tls.groups

按偏好顺序排列的 TLS key exchange group，以冒号分隔。

支持 `X25519`、`SECP256R1`、`SECP384R1` 和 `SECP521R1`，包括其常用 OpenSSL 和 NIST 别名。

为空时使用默认 TLS group。

### tls.control_wrap

OpenVPN 控制通道封装。

等价于 OpenVPN `tls-auth`、`tls-crypt` 和 `tls-crypt-v2`。

为空时禁用。

### tls.control_wrap.type

控制通道封装类型，可选值为 `tls_auth`、`tls_crypt` 或 `tls_crypt_v2`。

### tls.control_wrap.key

控制通道封装密钥内容。

与 `tls.control_wrap.key_path` 冲突。

### tls.control_wrap.key_path

控制通道封装密钥路径。

与 `tls.control_wrap.key` 冲突。

### tls.control_wrap.direction

`tls-auth` 密钥方向，可选值为 `server` 或 `client`。

仅当 `tls.control_wrap.type` 为 `tls_auth` 时可用。为空时双向使用密钥。

### cipher

`static_key` 模式使用的数据通道 cipher。

为空时使用上游静态密钥模式的默认值 `BF-CBC`。`BF-CBC` 是采用 64 位 block size
的旧 cipher；应尽可能显式配置服务器要求的 cipher。静态密钥 cipher 包括
`BF-CBC`、`CAST5-CBC`、`DES-CBC`、`DES-EDE-CBC`、`DES-EDE3-CBC`、
AES-CBC、ARIA-CBC、Camellia-CBC 系列，以及 `SEED-CBC`、`SM4-CBC` 和 `NONE`。

仅在 `static_key` 模式下可用。`NONE` 不提供机密性。

### data_ciphers

允许的 OpenVPN 数据通道 cipher。

仅在 TLS 模式下可用。

默认使用 `AES-256-GCM`、`AES-128-GCM` 和 `CHACHA20-POLY1305`。

AES-GCM 系列还包括 `AES-192-GCM`。保留的 cipher 包括 AES、ARIA、Camellia、DES、Blowfish、CAST5、SEED 和 SM4 的 CBC、CFB、OFB 形式，以及 `NONE`。CFB 和 OFB 仅可用于 TLS 模式。旧 cipher 只能提供较弱的机密性或完全不加密，因此默认不启用。

### data_ciphers_fallback

用于不支持 cipher 协商的对端的数据通道 cipher。

默认禁用。

仅在 TLS 模式下可用。

### auth

OpenVPN 数据通道认证摘要。

默认使用 `SHA1`，仅应用于非 AEAD 数据 cipher 和 `tls_auth`。

为兼容既有服务器，显式配置时仍可使用 `MD5` 和 `RIPEMD160` 等旧摘要。

### mss_fix

OpenVPN UDP packet 的最大大小，用于限制通过隧道发送的 TCP 连接 MSS。

这可以避免 TCP packet 在 OpenVPN 封装后超过 path MTU。

为空时使用上游 OpenVPN 默认值：配置了 `fragment` 时使用其值；否则默认 tunnel MTU 使用 `1492`，自定义 tunnel MTU 使用该 MTU。

### mss_fix_disabled

禁用 MSS 限制，包括默认限制。

与 `mss_fix` 和 `mss_fix_mode` 冲突。

### mss_fix_mode

显式 `mss_fix` 的 OpenVPN MSS 计算模式，`mtu` 或 `fixed` 之一。

空值使用普通的 OpenVPN 封装开销计算。`mtu` 还会计算外层 IP 和 UDP/TCP 传输头；`fixed` 将 `mss_fix` 视为内层 IPv4 数据包大小。

需要 `mss_fix`。

### fragment

用于 OpenVPN 数据通道 fragmentation 的最大 OpenVPN UDP packet 大小。

设为 `0` 时禁用。非零值必须至少为 `68`。

与 TCP 传输冲突。

### replay_window

UDP 数据通道重放窗口大小。默认使用 `64`，最大值为 `65536`。

TCP 始终要求数据包 ID 严格连续。

### replay_window_time

UDP 数据通道重放窗口时长。默认使用 `15s`，最大值为 `10m`。

该值必须使用整秒。

### compression

OpenVPN `compress` framing 模式，可选值为 `none`、`no`、`lz4`、`lz4-v2`、`stub`、`stub-v2`、`disabled` 或 `off`。

默认禁用。

Compression 可能削弱流量机密性。仅在需要 framing 兼容性时使用 `stub` 或 `stub-v2`。

### compression_lzo

OpenVPN `comp-lzo` 模式，可选值为 `none`、`no`、`yes`、`adaptive`、`asym`、`disabled` 或 `off`。

默认禁用。

Compression 可能削弱流量机密性。仅在服务器要求时启用。

### allow_compression

服务器推送的 compression 策略，可选值为 `no`、`asym` 或 `yes`。

默认使用 `no`，仅允许 compression stub framing。`asym` 接受来自服务器的 compressed packet，但不压缩出站 packet。为兼容 OpenVPN 2.7，`yes` 作为 `asym` 的旧别名接受；客户端绝不会发送 compressed packet。

当设为 `no` 时，与通过 `compression` 或 `compression_lzo` 启用的非 stub compression 冲突。

### route_no_pull

忽略服务器推送的 route、DNS 和 DHCP 设置、route metric、`redirect-gateway`、
`redirect-private`、`block-ipv6` 和 `block-outside-dns`。

仍会使用接口配置、topology、tunnel MTU、`route-gateway` 和本地配置的 route。

默认禁用。

### pull_filters

服务器推送选项的有序 pull filter 列表。

应用第一个 `text` 为完整推送选项大小写敏感前缀的 filter。未匹配任何 filter 的选项会被接受。

### pull_filters.action

==必填==

Filter action，可选值为 `accept`、`ignore` 或 `reject`。

`accept` 应用选项，`ignore` 丢弃选项，`reject` 终止连接。

### pull_filters.text

==必填==

用于匹配推送选项名称和值的大小写敏感前缀。

例如，`route ` 会匹配推送的 IPv4 route 选项，但不会匹配 `route-gateway`。

### routes

sing-box 路由优先选择此 OpenVPN endpoint 的 IPv4 和 IPv6 前缀。

这些 route 会与从服务器接受的 route 一起使用。

它们不会安装操作系统路由。请通过 sing-box 路由规则或 endpoint 的首选路由行为选择此 endpoint。

### route_gateway

通过 OpenVPN endpoint 路由的 IPv4 gateway。

为空时使用从服务器接收的 VPN gateway。

该值仅为兼容 OpenVPN 配置而保留；endpoint 的路由偏好只按前缀判断，不会安装系统 gateway 路由。

### route_metric

通过 OpenVPN endpoint 路由的默认 metric。

设为 `0` 时使用平台默认值。

该值仅为兼容 OpenVPN 配置而保留，不会安装系统路由。

### redirect_gateway

在 sing-box 路由中对所有 IPv4 目的地优先选择 OpenVPN endpoint。

默认禁用。

这不会安装操作系统默认路由。

### redirect_gateway_flags

OpenVPN `redirect-gateway` flag。

`!ipv4` 禁用 IPv4 偏好，`def1` 使用两个 `/1` 前缀表示，`ipv6` 还会优先选择上游特定的 IPv6 前缀。OpenVPN 控制连接始终使用其配置的出站拨号器，不经过 endpoint 路由，因此 `local` 和 `autolocal` 不需要系统路由例外。由于 sing-box 不会把推送的 DHCP 或 DNS 设置安装到操作系统，`bypass-dhcp` 和 `bypass-dns` 不适用。`block-local` 不受支持，因为 endpoint 没有可跨平台获取物理默认网关的来源，无法保留网关例外。

默认为空。

### redirect_private

接受 `redirect_gateway_flags`，但不添加默认路由偏好。单独推送或配置的路由仍会影响 endpoint 的首选地址，但不会安装操作系统路由。

默认禁用。

### block_ipv6

在本地拒绝 IPv6 流量，而不是通过 VPN 发送。

默认禁用。

### ping_interval

客户端未向服务器发送任何 packet 时，发送 data channel ping 的间隔。

服务器推送的 OpenVPN `ping` 值优先于该值。

该值必须使用整秒。

默认禁用。

### ping_restart

客户端未收到任何 packet 后重新连接服务器的时间。

服务器推送的 OpenVPN `ping-restart` 值优先于该值。

该值必须使用整秒。

为空时，启用了 pull 的 UDP 连接会使用 `120s`，直到服务器推送其他值。TCP 不使用默认接收超时。

### ping_restart_disabled

禁用初始 `120s` UDP 拉取超时和本地配置的 ping 重启超时。

与 `ping_restart` 冲突。

### renegotiate_interval

OpenVPN TLS 重新协商间隔。

为空时使用 OpenVPN 默认值 `1h`。

### renegotiate_disabled

禁用基于时间的 TLS 重新协商，包括默认间隔。

与 `renegotiate_interval` 冲突。

### renegotiate_bytes

传输指定字节数后重新协商数据通道密钥。`0` 使用与密码算法相关的 OpenVPN 默认值。

### renegotiate_packets

传输指定数据包数后重新协商数据通道密钥。`0` 使用与密码算法相关的 OpenVPN 默认值。

### tls_timeout

TLS 控制数据包的初始重传超时。为空时使用 OpenVPN 默认值 `2s`。

### handshake_window

初始 TLS 握手及每次重新协商的最长允许时间。为空时使用 OpenVPN 默认值 `1m`。

### explicit_exit_notify

关闭 UDP 连接时发送的 OpenVPN exit notification 数量。

Notification 之间间隔一秒。设为 `0` 时禁用。

### system

使用系统接口。

需要权限，且不能与现有系统接口冲突。

endpoint 会配置接口地址和 MTU，但不会安装操作系统路由或 DNS 设置。

禁用时，sing-box 使用内部网络栈。

### name

系统接口的自定义接口名称。

默认使用自动生成的 `ovpn` 接口名称。

### mtu

OpenVPN 接口 MTU。

为空时使用服务器推送的 MTU；收到服务器配置前使用 `1500`。

## UDP NAT 字段

参阅 [UDP NAT 字段](/zh/configuration/shared/udp-nat/)。

## 拨号字段

参阅[拨号字段](/zh/configuration/shared/dial/)。

## 交互式认证

在 sing-box dashboard 或任意 sing-box 图形客户端的 `工具` > `端点` 中认证和管理 endpoint。
