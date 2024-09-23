---
icon: material/alert-decagram
---

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
  "certificate": [],
  "certificate_path": "",
  "key": [],
  "key_path": "",
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
    "pq_signature_schemes_enabled": false,
    "dynamic_record_sizing_disabled": false,
    "key": [],
    "key_path": ""
  },
  "reality": {
    "enabled": false,
    "handshake": {
      "server": "google.com",
      "server_port": 443,
      ...
      // 拨号字段
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
  "certificate": [],
  "certificate_path": "",
  "ech": {
    "enabled": false,
    "pq_signature_schemes_enabled": false,
    "dynamic_record_sizing_disabled": false,
    "config": [],
    "config_path": ""
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

启用的 TLS 1.0-1.2密码套件的列表。列表的顺序被忽略。请注意，TLS 1.3 的密码套件是不可配置的。

如果为空，则使用安全的默认列表。默认密码套件可能会随着时间的推移而改变。

#### certificate

服务器 PEM 证书行数组。

#### certificate_path

!!! note ""

    文件更改时将自动重新加载。

服务器 PEM 证书路径。

#### key

==仅服务器==

!!! note ""

    文件更改时将自动重新加载。

服务器 PEM 私钥行数组。

#### key_path

==仅服务器==

服务器 PEM 私钥路径。

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

## ECH 字段

ECH (Encrypted Client Hello) 是一个 TLS 扩展，它允许客户端加密其 ClientHello 的第一部分
信息。

ECH 配置和密钥可以通过 `sing-box generate ech-keypair [--pq-signature-schemes-enabled]` 生成。

#### pq_signature_schemes_enabled

启用对后量子对等证书签名方案的支持。

建议匹配 `sing-box generate ech-keypair` 的参数。

#### dynamic_record_sizing_disabled

禁用 TLS 记录的自适应大小调整。

如果为 true，则始终使用最大可能的 TLS 记录大小。
如果为 false，则可能会调整 TLS 记录的大小以尝试改善延迟。

#### key

==仅服务器==

ECH PEM 密钥行数组

#### key_path

==仅服务器==

!!! note ""

    文件更改时将自动重新加载。

ECH PEM 密钥路径

#### config

==仅客户端==

ECH PEM 配置行数组

如果为空，将尝试从 DNS 加载。

#### config_path

==仅客户端==

ECH PEM 配置路径

如果为空，将尝试从 DNS 加载。

### ACME 字段

#### domain

一组域名。

默认禁用 ACME。

#### data_directory

ACME 数据目录。

默认使用 `$XDG_DATA_HOME/certmagic|$HOME/.local/share/certmagic`。

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

EAB（外部帐户绑定）包含将 ACME 帐户绑定或映射到其他已知帐户所需的信息由 CA。

外部帐户绑定“用于将 ACME 帐户与非 ACME 系统中的现有帐户相关联，例如 CA 客户数据库。

为了启用 ACME 帐户绑定，运行 ACME 服务器的 CA 需要向 ACME 客户端提供 MAC 密钥和密钥标识符，使用 ACME 之外的一些机制。
§7.3.4

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

服务器与和客户端之间允许的最大时间差。

默认禁用检查。
