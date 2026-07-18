# OpenVPN 客户端

!!! question "自 sing-box 1.14.0 起"

## 结构

```json
{
  "type": "openvpn-client",
  "tag": "ovpn-client",

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
  "username": "",
  "password": "",
  "auth_retry": "none",
  "static_challenge": "",
  "static_challenge_echo": false,
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
  "data_ciphers": [],
  "data_ciphers_fallback": "",
  "auth": "",
  "mss_fix": 0,
  "fragment": 0,
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
  "ping_interval": "",
  "ping_restart": "",
  "renegotiate_interval": "",
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

### username

OpenVPN 用户名/密码认证的用户名。

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

### tls

==必填==

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

多个值会被组合，证书必须包含所有要求的用途。

默认禁用。

### tls.remote_certificate_eku

服务器证书所需的 Extended Key Usage，可选值为 `server` 或 `client`。

默认禁用。标准 OpenVPN 服务器证书用途检查仍然生效。

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

### data_ciphers

允许的 OpenVPN 数据通道 cipher。

默认使用 `AES-256-GCM`、`AES-128-GCM` 和 `CHACHA20-POLY1305`。

### data_ciphers_fallback

用于不支持 cipher 协商的对端的数据通道 cipher。

默认禁用。

### auth

OpenVPN 数据通道认证摘要。

默认使用 `SHA1`，仅应用于非 AEAD 数据 cipher 和 `tls_auth`。

### mss_fix

OpenVPN UDP packet 的最大大小，用于限制通过隧道发送的 TCP 连接 MSS。

这可以避免 TCP packet 在 OpenVPN 封装后超过 path MTU。

为空时使用上游 OpenVPN 默认值：配置了 `fragment` 时使用其值；否则默认 tunnel MTU 使用 `1492`，自定义 tunnel MTU 使用该 MTU。

### fragment

用于 OpenVPN 数据通道 fragmentation 的最大 OpenVPN UDP packet 大小。

设为 `0` 时禁用。非零值必须至少为 `68`。

与 TCP 传输冲突。

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

默认使用 `no`，仅允许 compression stub framing。`asym` 接受来自服务器的 compressed packet，但不压缩出站 packet。`yes` 允许双向 compression。

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

通过 OpenVPN endpoint 路由的 IPv4 和 IPv6 route prefix。

这些 route 会与从服务器接受的 route 一起使用。

### route_gateway

通过 OpenVPN endpoint 路由的 IPv4 gateway。

为空时使用从服务器接收的 VPN gateway。

### route_metric

通过 OpenVPN endpoint 路由的默认 metric。

设为 `0` 时使用平台默认值。

### redirect_gateway

通过 OpenVPN endpoint 路由所有 IPv4 流量。

默认禁用。

### redirect_gateway_flags

OpenVPN `redirect-gateway` flag。

`!ipv4` 禁用 IPv4 default route，`ipv6` 还会通过 endpoint 路由所有 IPv6 流量。接受其他 OpenVPN flag 以兼容配置，但它们不会改变 endpoint 路由。

默认为空。

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

### renegotiate_interval

OpenVPN TLS 重新协商间隔。

为空时使用 OpenVPN 默认值 `1h`。

### explicit_exit_notify

关闭 UDP 连接时发送的 OpenVPN exit notification 数量。

Notification 之间间隔一秒。设为 `0` 时禁用。

### system

使用系统接口。

需要权限，且不能与现有系统接口冲突。

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
