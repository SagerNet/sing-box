---
icon: material/new-box
---

!!! quote "sing-box 1.13.0 中的更改"

    :material-plus: [kernel_tx](#kernel_tx)
    :material-plus: [kernel_rx](#kernel_rx)
    :material-plus: [curve_preferences](#curve_preferences)
    :material-plus: [certificate_public_key_sha256](#certificate_public_key_sha256)
    :material-plus: [client_certificate](#client_certificate)
    :material-plus: [client_certificate_path](#client_certificate_path)
    :material-plus: [client_key](#client_key)
    :material-plus: [client_key_path](#client_key_path)
    :material-plus: [client_authentication](#client_authentication)
    :material-plus: [client_certificate_public_key_sha256](#client_certificate_public_key_sha256)

!!! quote "sing-box 1.12.0 中的更改"

    :material-plus: [fragment](#fragment)
    :material-plus: [fragment_fallback_delay](#fragment_fallback_delay)
    :material-plus: [record_fragment](#record_fragment)
    :material-delete-clock: [ech.pq_signature_schemes_enabled](#pq_signature_schemes_enabled)
    :material-delete-clock: [ech.dynamic_record_sizing_disabled](#dynamic_record_sizing_disabled)

!!! quote "sing-box 1.10.0 中的更改"

    :material-alert-decagram: [utls](#utls)  

### 入站

```json
{
  "enabled": true,
  "server_name": "",
  "alpn": [],
  "min_version": "",
  "max_version": "",
  "cipher_suites": [],
  "curve_preferences": [],
  "certificate": [],
  "certificate_path": "",
  "client_authentication": "",
  "client_certificate": [],
  "client_certificate_path": [],
  "client_certificate_public_key_sha256": [],
  "key": [],
  "key_path": "",
  "kernel_tx": false,
  "kernel_rx": false,
  "acme": {
    "domain": [],
    "data_directory": "",
    "default_server_name": "",
    "email": "",
    "provider": "",
    "disable_http_challenge": false,
    "disable_tls_alpn_challenge": false,
    "alternative_http_port": 0,
    "alternative_tls_port": 0,
    "external_account": {
      "key_id": "",
      "mac_key": ""
    },
    "dns01_challenge": {}
  },
  "ech": {
    "enabled": false,
    "key": [],
    "key_path": "",

    // 废弃的
    
    "pq_signature_schemes_enabled": false,
    "dynamic_record_sizing_disabled": false
  },
  "reality": {
    "enabled": false,
    "handshake": {
      "server": "google.com",
      "server_port": 443,
      
      ... // 拨号字段
    },
    "private_key": "UuMBgl7MXTPx9inmQp2UC7Jcnwc6XYbwDNebonM-FCc",
    "short_id": [
      "0123456789abcdef"
    ],
    "max_time_difference": "1m"
  }
}
```

### 出站

```json
{
  "enabled": true,
  "disable_sni": false,
  "server_name": "",
  "insecure": false,
  "alpn": [],
  "min_version": "",
  "max_version": "",
  "cipher_suites": [],
  "curve_preferences": [],
  "certificate": "",
  "certificate_path": "",
  "certificate_public_key_sha256": [],
  "client_certificate": [],
  "client_certificate_path": "",
  "client_key": [],
  "client_key_path": "",
  "fragment": false,
  "fragment_fallback_delay": "",
  "record_fragment": false,
  "ech": {
    "enabled": false,
    "config": [],
    "config_path": "",

    // 废弃的
    "pq_signature_schemes_enabled": false,
    "dynamic_record_sizing_disabled": false
  },
  "utls": {
    "enabled": false,
    "fingerprint": ""
  },
  "reality": {
    "enabled": false,
    "public_key": "jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0",
    "short_id": "0123456789abcdef"
  }
}
```

TLS 版本值：

* `1.0`
* `1.1`
* `1.2`
* `1.3`

密码套件值：

* `TLS_RSA_WITH_AES_128_CBC_SHA`
* `TLS_RSA_WITH_AES_256_CBC_SHA`
* `TLS_RSA_WITH_AES_128_GCM_SHA256`
* `TLS_RSA_WITH_AES_256_GCM_SHA384`
* `TLS_AES_128_GCM_SHA256`
* `TLS_AES_256_GCM_SHA384`
* `TLS_CHACHA20_POLY1305_SHA256`
* `TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA`
* `TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA`
* `TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA`
* `TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA`
* `TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256`
* `TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384`
* `TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256`
* `TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384`
* `TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256`
* `TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256`

!!! note ""

    当内容只有一项时，可以忽略 JSON 数组 [] 标签

### 字段

#### enabled

启用 TLS

#### disable_sni

==仅客户端==

不要在 ClientHello 中发送服务器名称.

#### server_name

用于验证返回证书上的主机名，除非设置不安全。

它还包含在 ClientHello 中以支持虚拟主机，除非它是 IP 地址。

#### insecure

==仅客户端==

接受任何服务器证书。

#### alpn

支持的应用层协议协商列表，按优先顺序排列。

如果两个对等点都支持 ALPN，则选择的协议将是此列表中的一个，如果没有相互支持的协议则连接将失败。

参阅 [Application-Layer Protocol Negotiation](https://en.wikipedia.org/wiki/Application-Layer_Protocol_Negotiation)。

#### min_version

可接受的最低 TLS 版本。

默认情况下，当前使用 TLS 1.2 作为客户端的最低要求。作为服务器时使用 TLS 1.0。

#### max_version

可接受的最大 TLS 版本。

默认情况下，当前最高版本为 TLS 1.3。

#### cipher_suites

启用的 TLS 1.0–1.2 密码套件列表。列表的顺序被忽略。请注意，TLS 1.3 的密码套件是不可配置的。

如果为空，则使用安全的默认列表。默认密码套件可能会随着时间的推移而改变。

#### curve_preferences

!!! question "自 sing-box 1.13.0 起"

支持的密钥交换机制集合。列表的顺序被忽略，密钥交换机制通过 Golang 的内部偏好顺序从此列表中选择。

可用值，同时也是默认列表：

* `P256`
* `P384`
* `P521`
* `X25519`
* `X25519MLKEM768`

#### certificate

服务器证书链行数组，PEM 格式。

#### certificate_path

!!! note ""

    文件更改时将自动重新加载。

服务器证书链路径，PEM 格式。

#### certificate_public_key_sha256

!!! question "自 sing-box 1.13.0 起"

==仅客户端==

服务器证书公钥的 SHA-256 哈希列表，base64 格式。

要生成证书公钥的 SHA-256 哈希，请使用以下命令：

```bash
# 对于证书文件
openssl x509 -in certificate.pem -pubkey -noout | openssl pkey -pubin -outform der | openssl dgst -sha256 -binary | openssl enc -base64

# 对于远程服务器的证书
echo | openssl s_client -servername example.com -connect example.com:443 2>/dev/null | openssl x509 -pubkey -noout | openssl pkey -pubin -outform der | openssl dgst -sha256 -binary | openssl enc -base64
```

#### client_certificate

!!! question "自 sing-box 1.13.0 起"

==仅客户端==

客户端证书链行数组，PEM 格式。

#### client_certificate_path

!!! question "自 sing-box 1.13.0 起"

==仅客户端==

客户端证书链路径，PEM 格式。

#### client_key

!!! question "自 sing-box 1.13.0 起"

==仅客户端==

客户端私钥行数组，PEM 格式。

#### client_key_path

!!! question "自 sing-box 1.13.0 起"

==仅客户端==

客户端私钥路径，PEM 格式。

#### key

==仅服务器==

!!! note ""

    文件更改时将自动重新加载。

服务器 PEM 私钥行数组。

#### key_path

==仅服务器==

!!! note ""

    文件更改时将自动重新加载。

服务器私钥路径，PEM 格式。

#### client_authentication

!!! question "自 sing-box 1.13.0 起"

==仅服务器==

要使用的客户端身份验证类型。

可用值：

* `no`（默认）
* `request`
* `require-any`
* `verify-if-given`
* `require-and-verify`

如果此选项设置为 `verify-if-given` 或 `require-and-verify`，
则需要 `client_certificate`、`client_certificate_path` 或 `client_certificate_public_key_sha256` 中的一个。

#### client_certificate

!!! question "自 sing-box 1.13.0 起"

==仅服务器==

客户端证书链行数组，PEM 格式。

#### client_certificate_path

!!! question "自 sing-box 1.13.0 起"

==仅服务器==

!!! note ""

    文件更改时将自动重新加载。

客户端证书链路径列表，PEM 格式。

#### client_certificate_public_key_sha256

!!! question "自 sing-box 1.13.0 起"

==仅服务器==

客户端证书公钥的 SHA-256 哈希列表，base64 格式。

要生成证书公钥的 SHA-256 哈希，请使用以下命令：

```bash
# 对于证书文件
openssl x509 -in certificate.pem -pubkey -noout | openssl pkey -pubin -outform der | openssl dgst -sha256 -binary | openssl enc -base64

# 对于远程服务器的证书
echo | openssl s_client -servername example.com -connect example.com:443 2>/dev/null | openssl x509 -pubkey -noout | openssl pkey -pubin -outform der | openssl dgst -sha256 -binary | openssl enc -base64
```

#### kernel_tx

!!! question "自 sing-box 1.13.0 起"

!!! quote ""

    仅支持 Linux 5.1+，如果可能，使用较新的内核。

!!! quote ""

    仅支持 TLS 1.3。

!!! warning ""

    kTLS TX 仅当 `splice(2)` 可用时（两端经过握手后必须为没有附加协议的 TCP 或 TLS）才能提高性能；否则肯定会降低性能。

启用内核 TLS 发送支持。

#### kernel_rx

!!! question "自 sing-box 1.13.0 起"

!!! quote ""

    仅支持 Linux 5.1+，如果可能，使用较新的内核。

!!! quote ""

    仅支持 TLS 1.3。

!!! failure ""

    即使使用 `splice(2)`，kTLS RX 也肯定会降低性能，因此不建议启用。

启用内核 TLS 接收支持。

## 自定义 TLS 支持

!!! info "QUIC 支持"

    只有 ECH 在 QUIC 中被支持.

#### utls

==仅客户端==

!!! failure ""

    没有证据表明 GFW 根据 TLS 客户端指纹检测并阻止服务器，并且，使用一个未经安全审查的不完美模拟可能带来安全隐患。

uTLS 是 "crypto/tls" 的一个分支，它提供了 ClientHello 指纹识别阻力。

可用的指纹值：

!!! warning "已在 sing-box 1.10.0 移除"

    一些旧 chrome 指纹已被删除，并将会退到 chrome：

    :material-close: chrome_psk  
    :material-close: chrome_psk_shuffle  
    :material-close: chrome_padding_psk_shuffle  
    :material-close: chrome_pq  
    :material-close: chrome_pq_psk

* chrome
* firefox
* edge
* safari
* 360
* qq
* ios
* android
* random
* randomized

默认使用 chrome 指纹。

### ECH 字段

ECH (Encrypted Client Hello) 是一个 TLS 扩展，它允许客户端加密其 ClientHello 的第一部分信息。

ECH 密钥和配置可以通过 `sing-box generate ech-keypair` 生成。

#### pq_signature_schemes_enabled

!!! failure "已在 sing-box 1.12.0 废弃"

    ECH 支持已在 sing-box 1.12.0 迁移至使用标准库，但标准库不支持后量子对等证书签名方案，因此 `pq_signature_schemes_enabled` 已被弃用且不再工作。

启用对后量子对等证书签名方案的支持。

#### dynamic_record_sizing_disabled

!!! failure "已在 sing-box 1.12.0 废弃"

    `dynamic_record_sizing_disabled` 与 ECH 无关，是错误添加的，现已弃用且不再工作。

禁用 TLS 记录的自适应大小调整。

当为 true 时，总是使用最大可能的 TLS 记录大小。
当为 false 时，可能会调整 TLS 记录的大小以尝试改善延迟。

#### key

==仅服务器==

ECH 密钥行数组，PEM 格式。

#### key_path

==仅服务器==

!!! note ""

    文件更改时将自动重新加载。

ECH 密钥路径，PEM 格式。

#### config

==仅客户端==

ECH 配置行数组，PEM 格式。

如果为空，将尝试从 DNS 加载。

#### config_path

==仅客户端==

ECH 配置路径，PEM 格式。

如果为空，将尝试从 DNS 加载。

#### fragment

!!! question "自 sing-box 1.12.0 起"

==仅客户端==

通过分段 TLS 握手数据包来绕过防火墙。

此功能旨在规避基于**明文数据包匹配**的简单防火墙，不应该用于规避真正的审查。

由于性能不佳，请首先尝试 `record_fragment`，且仅应用于已知被阻止的服务器名称。

在 Linux、Apple 平台和（需要管理员权限的）Windows 系统上，
可以自动检测等待时间。否则，将回退到
等待 `fragment_fallback_delay` 指定的固定时间。

此外，如果实际等待时间少于 20ms，也会回退到等待固定时间，
因为目标被认为是本地的或在透明代理后面。

#### fragment_fallback_delay

!!! question "自 sing-box 1.12.0 起"

==仅客户端==

当 TLS 分段无法自动确定等待时间时使用的回退值。

默认使用 `500ms`。

#### record_fragment

!!! question "自 sing-box 1.12.0 起"

==仅客户端==

将 TLS 握手分段为多个 TLS 记录以绕过防火墙。

### ACME 字段

#### domain

域名列表。

如果为空则禁用 ACME。

#### data_directory

ACME 数据存储目录。

如果为空则使用 `$XDG_DATA_HOME/certmagic|$HOME/.local/share/certmagic`。

#### default_server_name

如果 ClientHello 的 ServerName 字段为空，则选择证书时要使用的服务器名称。

#### email

创建或选择现有 ACME 服务器帐户时使用的电子邮件地址。

#### provider

要使用的 ACME CA 供应商。

| 值                  | 供应商           |
|--------------------|---------------|
| `letsencrypt (默认)` | Let's Encrypt |
| `zerossl`          | ZeroSSL       |
| `https://...`      | 自定义           |

#### disable_http_challenge

禁用所有 HTTP 质询。

#### disable_tls_alpn_challenge

禁用所有 TLS-ALPN 质询。

#### alternative_http_port

用于 ACME HTTP 质询的备用端口；如果非空，将使用此端口而不是 80 来启动 HTTP 质询的侦听器。

#### alternative_tls_port

用于 ACME TLS-ALPN 质询的备用端口； 系统必须将 443 转发到此端口以使质询成功。

#### external_account

EAB（外部帐户绑定）包含将 ACME 帐户绑定或映射到 CA 已知的其他帐户所需的信息。

外部帐户绑定"用于将 ACME 帐户与非 ACME 系统中的现有帐户相关联，例如 CA 客户数据库。

为了启用 ACME 帐户绑定，运行 ACME 服务器的 CA 需要使用 ACME 之外的某种机制向 ACME 客户端提供 MAC 密钥和密钥标识符。§7.3.4

#### external_account.key_id

密钥标识符。

#### external_account.mac_key

MAC 密钥。

#### dns01_challenge

ACME DNS01 验证字段。如果配置，将禁用其他验证方法。

参阅 [DNS01 验证字段](/configuration/shared/dns01_challenge/)。

### Reality 字段

#### handshake

==仅服务器==

==必填==

握手服务器地址和 [拨号参数](/zh/configuration/shared/dial/)。

#### private_key

==仅服务器==

==必填==

私钥，由 `sing-box generate reality-keypair` 生成。

#### public_key

==仅客户端==

==必填==

公钥，由 `sing-box generate reality-keypair` 生成。

#### short_id

==必填==

一个零到八位的十六进制字符串。

#### max_time_difference

==仅服务器==

服务器和客户端之间的最大时间差。

如果为空则禁用检查。
