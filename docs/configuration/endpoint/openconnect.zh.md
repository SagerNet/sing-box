# OpenConnect 客户端

!!! question "自 sing-box 1.14.0 起"

==仅客户端==

## 结构

```json
{
  "type": "openconnect",
  "tag": "oc-client",

  "system": false,
  "name": "",

  ... // UDP NAT 字段

  "server": "vpn.example.com",
  "flavor": "anyconnect",
  "username": "",
  "password": "",
  "auth_group": "",
  "cookie": "",
  "token": {
    "mode": "",
    "secret": "",
    "secret_path": "",
    "pin": "",
    "password": "",
    "device_id": "",
    "counter": 0
  },
  "reported_os": "",
  "user_agent": "",
  "version": "",
  "local_hostname": "",
  "mobile": {
    "platform_version": "",
    "device_type": "",
    "device_unique_id": ""
  },
  "csd": {
    "wrapper_path": ""
  },
  "hip": {
    "wrapper_path": ""
  },
  "tncc": {
    "wrapper_path": "",
    "device_id": "",
    "user_agent": "",
    "machine_identification_enabled": false,
    "certificates": [
      {
        "certificate": [],
        "certificate_path": ""
      }
    ]
  },
  "fortinet_host_check": {
    "hostcheck": "",
    "check_virtual_desktop": ""
  },
  "no_udp": false,
  "dtls_local_port": 0,
  "compression_disabled": false,
  "compression_mode": "",
  "ipv6_disabled": false,
  "http_keepalive_disabled": false,
  "xml_post_disabled": false,
  "external_auth_disabled": false,
  "password_authentication_disabled": false,
  "tcp_keep_alive_enabled": false,
  "pfs": false,
  "mtu": 0,
  "base_mtu": 0,
  "dpd_interval": "",
  "reconnect_timeout": "",
  "trojan_interval": "",
  "queue_length": 0,
  "allow_insecure_crypto": false,
  "tls": {
    "insecure": false,
    "server_name": "",
    "peer_fingerprint": [],
    "system_trust_disabled": false,
    "certificate_authority": [],
    "certificate_authority_path": "",
    "client_certificate": [],
    "client_certificate_path": "",
    "client_key": [],
    "client_key_path": "",
    "client_key_password": "",
    "mca_certificate": [],
    "mca_certificate_path": "",
    "mca_key": [],
    "mca_key_path": "",
    "mca_key_password": ""
  },
  "form_entries": [
    {
      "form_id": "",
      "submission_key": "",
      "name": "",
      "value": "",
      "promote": false
    }
  ],

  ... // 拨号字段
}
```

!!! note ""

    当内容只有一项时，可以忽略 JSON 数组 [] 标签。

## 字段

### system

使用系统接口。

需要权限，且不能与现有系统接口冲突。

禁用时，sing-box 使用内部网络栈。

### name

系统接口的自定义接口名称。

默认使用自动生成的 `oc` 接口名称。

### server

==必填==

OpenConnect VPN 服务器 HTTPS URL。

省略协议时会添加 `https://`。不支持 URL 用户信息、查询和片段。

### flavor

OpenConnect 协议 flavor，可选值为 `anyconnect`、`gp`、`fortinet`、`f5`、`pulse` 或 `nc`。

默认使用 `anyconnect`。

### username

用于填充匹配认证表单字段的用户名。

### password

用于填充匹配认证表单字段的密码。

### auth_group

认证组，用于在所选 flavor 支持时预选匹配的组、realm、domain 或 gateway 选项。

### cookie

用于跳过凭据提示并直接连接的现有认证会话。

接受的格式取决于 `flavor`：

- `anyconnect`：`webvpn` 值，或包含 `webvpn` 的分号分隔 cookie 列表。
- `gp`：GlobalProtect 认证返回的完整 authenticated query string。
- `nc`：`DSID` 值，或包含 `DSID` 的分号分隔 cookie 列表。
- `pulse`：原始 Pulse 认证 cookie 值。
- `f5`：`MRHSession` 值，或包含 `MRHSession` 及可选 `F5_ST` 的分号分隔 cookie 列表。
- `fortinet`：`SVPNCOOKIE` 值，或包含 `SVPNCOOKIE` 的分号分隔 cookie 列表。

如果服务器拒绝提供的会话，将尝试正常认证。

### token

用于自动回答匹配 token 字段或进行 HTTP Bearer 认证的 token 配置。

必须设置 `token.secret` 或 `token.secret_path` 之一。

### token.mode

==必填==

Token 模式，可选值为：

- `totp`：基于时间的一次性密码。
- `hotp`：基于 HMAC 的一次性密码。
- `stoken`：RSA SecurID 软件 token。
- `oidc`：用于 HTTP Bearer 认证的 OIDC access token。

### token.secret

软件 token 密钥。

对于 `totp` 和 `hotp`，可以是 Base32 密钥、带 `base32:` 前缀的密钥或类型匹配的 `otpauth://` URI。

对于 `stoken`，这是编码后的 RSA SecurID CTF token 内容。

对于 `oidc`，这是 access token 值。仅在 VPN 服务器请求 HTTP Bearer 认证后发送。

与 `token.secret_path` 冲突。

### token.secret_path

软件 token 密钥或 OIDC access token 的路径。

与 `token.secret` 冲突。

### token.pin

`stoken` 模式的 RSA SecurID PIN。

### token.password

`stoken` 模式下用于解密受密码保护的 RSA SecurID token 的密码。

### token.device_id

`stoken` 模式下用于解密设备绑定 RSA SecurID token 的设备 ID。

### token.counter

`hotp` 模式的初始计数器。

为零时，如果 `otpauth://` URI 中存在计数器，则使用该计数器；否则从零开始。

### reported_os

所选 flavor 支持时向 VPN 服务器报告的操作系统标识。

对于 `anyconnect`、`gp` 和 `pulse`，支持的值为 `linux`、`linux-64`、`win`、`mac-intel`、`android` 和 `apple-ios`。

默认值根据系统平台选择：Windows 使用 `win`，macOS 使用 `mac-intel`，Android 使用 `android`，iOS 使用 `apple-ios`，其他 64 位或 32 位系统使用 `linux-64` 或 `linux`。

### user_agent

所选 flavor 支持时向 VPN 服务器报告的 User-Agent。

默认值由 flavor 决定。AnyConnect、Network Connect、Pulse 和 F5 使用 `AnyConnect-compatible OpenConnect VPN Agent v9.21`；GlobalProtect 使用 `PAN GlobalProtect`；Fortinet 使用 `Mozilla/5.0 SV1`。

### version

所选 flavor 支持时，与 `user_agent` 分开报告的客户端版本。

默认使用 `v9.21`。当前用于 AnyConnect XML 认证。

### local_hostname

所选 flavor 支持时向 VPN 服务器报告的本地主机名。

默认使用系统主机名；无法获取时使用 `localhost`。

### mobile

AnyConnect 移动客户端身份。配置时三个字段均为必填，并会在 XML 认证和隧道建立阶段报告。

### mobile.platform_version

向 AnyConnect 服务器报告的移动操作系统版本。

### mobile.device_type

向 AnyConnect 服务器报告的移动设备型号或类型。

### mobile.device_unique_id

向 AnyConnect 服务器报告的移动设备标识符。

### csd

AnyConnect CSD/host scan 合规性选项。

服务器请求 CSD 时，默认使用内置 CSD 处理。

### csd.wrapper_path

外部 AnyConnect CSD wrapper 可执行文件的路径。

为空时使用内置 CSD 处理。

### hip

GlobalProtect HIP 检查和报告选项。

服务器请求 HIP 时，默认使用内置 HIP 报告。

### hip.wrapper_path

外部 GlobalProtect HIP report wrapper 可执行文件的路径。

为空时使用内置 HIP 报告。

### tncc

Network Connect TNCC 合规性选项。

服务器请求 TNCC 时，默认使用内置 TNCC 处理。

### tncc.wrapper_path

外部 Network Connect TNCC wrapper 可执行文件的路径。

为空时使用内置 TNCC 处理。

与 `tncc.device_id`、`tncc.user_agent`、`tncc.machine_identification_enabled` 和 `tncc.certificates` 冲突。

### tncc.device_id

内置 TNCC 处理程序报告的设备 ID。

与 `tncc.wrapper_path` 冲突。

### tncc.user_agent

内置 TNCC 处理程序使用的 User-Agent。

默认使用 `Neoteris HC Http`。

与 `tncc.wrapper_path` 冲突。

### tncc.machine_identification_enabled

启用内置 TNCC 机器标识，包括平台、主机名和观测到的 MAC 地址。

与 `tncc.wrapper_path` 冲突。

### tncc.certificates

内置 TNCC 处理程序用于回答证书请求的机器证书。

需要启用 `tncc.machine_identification_enabled`。

与 `tncc.wrapper_path` 冲突。

### tncc.certificates.certificate

PEM 格式的 TNCC 机器证书内容。

与 `tncc.certificates.certificate_path` 冲突。

### tncc.certificates.certificate_path

PEM 格式的 TNCC 机器证书路径。

与 `tncc.certificates.certificate` 冲突。

### fortinet_host_check

Fortinet hostcheck 结果覆盖选项。

默认禁用 hostcheck。仅当 `fortinet_host_check.hostcheck` 非空时启用。不会自动收集操作系统、安全产品或网络接口信息。

启用后，如果成功的 Fortinet 登录响应要求 hostcheck，将在使用 VPN 会话前向服务器提交两个配置值。这些值不经修改，作为 `application/x-www-form-urlencoded` 字段发送。

部分 Fortinet 服务器只会要求可识别的 FortiClient User-Agent 执行 hostcheck。服务器策略有要求时请配置 `user_agent`。

### fortinet_host_check.hostcheck

Fortinet hostcheck 结果字符串。

通常格式为 `<security-status>,<os-version>`，例如 `0100,10.0.19042`。`security-status` 包含四个 `0` 或 `1` 字符，依次表示第三方防火墙、第三方杀毒软件、FortiClient 防火墙和 FortiClient 杀毒软件。

空值会禁用 Fortinet hostcheck，即使配置了 `fortinet_host_check.check_virtual_desktop`。

### fortinet_host_check.check_virtual_desktop

Fortinet virtual desktop 检查结果字符串。

FortiClient 通常发送以冒号分隔的 MAC 地址，多个地址使用 `|` 连接，例如 `74:78:27:4d:81:93|84:1b:77:3a:95:84`。启用 hostcheck 时，空值会作为空字段提交。

### no_udp

禁用 DTLS 或 ESP 辅助数据通道，仅使用 TLS 数据通道。

### dtls_local_port

直连 DTLS 或 ESP 辅助数据通道使用的本地 UDP 端口。

默认自动选择临时端口。

### compression_disabled

禁用 AnyConnect 压缩协商。

默认情况下，当服务器支持时，CSTP 和 DTLS 会协商无状态 `oc-lz4` 和 `lzs` 压缩。

当攻击者能够影响通过 VPN 隧道发送的明文时，压缩可能削弱流量机密性。

与设置为 `all` 的 `compression_mode` 冲突。

### compression_mode

AnyConnect 压缩模式，可选值为：

- `stateless`：声明支持无状态 `oc-lz4` 和 `lzs` 压缩。
- `all`：额外声明支持 CSTP 有状态 `deflate` 压缩。

默认使用 `stateless`。即使选择 `all`，DTLS 也始终使用无状态压缩。

有状态压缩存在额外的流量机密性风险，仅应在 VPN 服务器需要时启用。

### ipv6_disabled

禁用请求和使用 IPv6 隧道配置。

### http_keepalive_disabled

在认证和配置请求中禁用 HTTP 连接复用。

### xml_post_disabled

禁用 AnyConnect XML POST 认证，并直接使用旧版 GET 流程开始认证。

### external_auth_disabled

禁用 AnyConnect 和 GlobalProtect 的 SSO、SAML 等外部浏览器认证。

启用时不会向服务器声明外部认证支持，并会拒绝意外收到的外部认证请求。

### password_authentication_disabled

如果服务器返回非成功的认证表单，则中止 AnyConnect 认证，与 OpenConnect `--no-passwd` 行为一致。

此选项不影响其他 flavor，也不影响由 `cookie` 提供的会话。

### tcp_keep_alive_enabled

为直接 VPN 服务器连接启用 TCP keep alive。

默认禁用以匹配 OpenConnect。设置 `tcp_keep_alive` 或 `tcp_keep_alive_interval` 也会启用，无需同时设置此字段。启用但未设置这两个时间值时，保留操作系统的 TCP keep alive 时间设置。

与 `disable_tcp_keep_alive` 冲突。

### pfs

要求 TLS 1.2 及更早版本使用具有前向保密性的 TLS 密码套件。

默认禁用，以兼容需要 RSA 密钥交换的 VPN 服务器。此选项不会启用已弃用的密码套件；旧版加密支持参阅 `allow_insecure_crypto`。

### mtu

首选隧道 MTU。

所有 flavor 协商的 MTU 都不会超过此值。对于 AnyConnect，此值还会发送给服务器。GlobalProtect、F5 和 Fortinet 会先扣除各自的协议开销，再将结果作为隧道 MTU。

非零值小于 `576` 时按 `576` 处理。最大值为 `65535`。

### base_mtu

扣除外层 IP、传输和协议开销后，用于计算 AnyConnect、GlobalProtect、F5 和 Fortinet 隧道 MTU 的基础路径 MTU。

默认使用 `1406`。

这些 flavor 会将小于 `1280` 的值按 `1280` 处理。最大值为 `65535`。

### dpd_interval

覆盖 Dead Peer Detection 间隔。

默认使用服务器提供或 flavor 特定的间隔。

大于零且小于 `2s` 的值按 `2s` 处理。值不得为负数。

### reconnect_timeout

重连尝试失败后允许累计使用的最大退避时间。断线后的第一次重连会立即开始，且此超时不会取消已经进行中的尝试。

默认使用 `300s`。

值不得为负数。

### trojan_interval

覆盖 GlobalProtect HIP report 或 Network Connect TNCC check 的执行间隔。

默认使用服务器提供的间隔。服务器未提供时，GlobalProtect 使用 `1h`。

值不得为负数。

### queue_length

VPN transport 与隧道接口之间的入站和出站数据包队列长度。

默认使用 `32`。队列已满时会施加反压并等待消费者腾出空间，不会丢弃已排队的数据包。

### allow_insecure_crypto

启用旧版 VPN 服务器所需的弱 TLS 和 DTLS 密码套件及 TLS 1.0 兼容性。

默认禁用；未启用时会拒绝低于 TLS 1.2 的版本。此选项不会禁用服务器证书验证。

### tls

OpenConnect TLS 配置。

### tls.insecure

禁用 VPN 服务器证书和主机名验证。

默认禁用。启用后，主动攻击者可以冒充 VPN 服务器。应尽可能使用 `tls.certificate_authority` 或 `tls.peer_fingerprint`。

### tls.server_name

用于 TLS SNI 和证书主机名验证的服务器名称。

默认使用 `server` 中的主机名。

### tls.peer_fingerprint

允许的服务器证书指纹。可以指定单个字符串或列表。

支持的格式：

- 与 OpenConnect `--servercert` 兼容的无前缀 SHA-1 证书指纹。
- `sha1:<hex>`：SHA-1 SPKI 指纹。
- `sha256:<hex>`：SHA-256 SPKI 指纹。
- `pin-sha256:<base64>`：Base64 编码的 SHA-256 SPKI pin。

每种格式的编码指纹均可缩写为至少四个字符的前缀。配置后，对端证书必须匹配其中一个指纹；匹配的指纹可以授权未通过其他方式信任的证书。

### tls.system_trust_disabled

禁用系统 CA 证书池。

启用时，使用 `tls.certificate_authority` 或 `tls.peer_fingerprint` 建立信任。

### tls.certificate_authority

PEM 格式的附加受信任 CA 证书内容。

这些证书会添加到系统证书池。

与 `tls.certificate_authority_path` 冲突。

### tls.certificate_authority_path

PEM 格式的附加受信任 CA 证书路径。

这些证书会添加到系统证书池。

与 `tls.certificate_authority` 冲突。

### tls.client_certificate

PEM 格式的客户端证书链内容。

与 `tls.client_certificate_path` 冲突。

### tls.client_certificate_path

PEM 格式的客户端证书链路径。

与 `tls.client_certificate` 冲突。

### tls.client_key

PEM 格式的客户端私钥内容。

与 `tls.client_key_path` 冲突。

### tls.client_key_path

PEM 格式的客户端私钥路径。

与 `tls.client_key` 冲突。

客户端证书和私钥必须同时设置或同时为空。

### tls.client_key_password

加密客户端私钥的密码。

### tls.mca_certificate

PEM 格式的 AnyConnect 多证书认证（MCA）证书链内容。

与 `tls.mca_certificate_path` 冲突。

### tls.mca_certificate_path

PEM 格式的 AnyConnect 多证书认证（MCA）证书链路径。

与 `tls.mca_certificate` 冲突。

### tls.mca_key

PEM 格式的 AnyConnect 多证书认证（MCA）私钥内容。

与 `tls.mca_key_path` 冲突。

### tls.mca_key_path

PEM 格式的 AnyConnect 多证书认证（MCA）私钥路径。

与 `tls.mca_key` 冲突。

MCA 证书和私钥必须同时设置或同时为空。

### tls.mca_key_password

加密 MCA 私钥的密码。

### form_entries

认证表单字段覆盖。

设置 `submission_key` 时按该字段匹配，否则按 `form_id` 和 `name` 的组合匹配。后面的匹配项优先。

### form_entries.form_id

`form_entries.submission_key` 为空时，与 `form_entries.name` 一起使用的认证表单标识符。

### form_entries.submission_key

认证字段提交键。

`form_entries.submission_key` 或 `form_entries.form_id` 与 `form_entries.name` 的组合之一必填。

### form_entries.name

`form_entries.submission_key` 为空时，与 `form_entries.form_id` 一起使用的认证字段名称。

### form_entries.value

自动提供给匹配认证字段的值。

与 `form_entries.promote` 冲突。

### form_entries.promote

交互询问匹配的认证字段，而不是自动提供值。

与 `form_entries.value` 冲突。

## UDP NAT 字段

参阅 [UDP NAT 字段](/zh/configuration/shared/udp-nat/)。

## 拨号字段

参阅[拨号字段](/zh/configuration/shared/dial/)了解详情。

## 交互式认证

在 sing-box dashboard 或任意 sing-box 图形客户端的 `工具` > `端点` 中认证和管理 endpoint。

## DNS

推送的 DNS 设置不会安装到操作系统中。配置 [OpenConnect DNS 服务器](/zh/configuration/dns/server/openconnect/) 以通过 sing-box 使用这些设置。
