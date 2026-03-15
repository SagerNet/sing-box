---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

# ACME

!!! quote ""

    需要 `with_acme` 构建标签。

!!! info ""

    该提供者用于替代已废弃的内联 `tls.acme` 选项。
    下文中未标记版本的字段均从 `tls.acme` 迁移而来；
    标记 `自 sing-box 1.14.0 起` 的字段为 1.14.0 新增字段。

### 结构

```json
{
  "type": "acme",
  "tag": "",

  "domain": [],
  "data_directory": "",
  "default_server_name": "",
  "email": "",
  "provider": "",
  "test_ca": "",
  "account_key": "",
  "trusted_roots_pem_files": [],
  "acme_timeout": "",
  "profile": "",
  "certificate_lifetime": "",
  "disable_http_challenge": false,
  "disable_tls_alpn_challenge": false,
  "alternative_http_port": 0,
  "alternative_tls_port": 0,
  "bind_host": "",
  "external_account": {
    "key_id": "",
    "mac_key": ""
  },
  "dns01_challenge": {},
  "preferred_chains": {
    "smallest": false,
    "root_common_name": [],
    "any_common_name": []
  },
  "key_type": "",
  "reuse_private_keys": false,
  "must_staple": false,
  "renewal_window_ratio": 0.0,
  "disable_ocsp_stapling": false,
  "ocsp_overrides": {}
}
```

### 字段

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

当 `provider` 为 `zerossl` 时，如果设置了 `email` 且未设置 `external_account`，
sing-box 会自动向 ZeroSSL 请求 EAB 凭据。

当 `provider` 为 `zerossl` 时，必须至少设置 `external_account`、`email` 或 `account_key` 之一。

#### test_ca

!!! question "自 sing-box 1.14.0 起"

重试时使用的测试 ACME directory URL。

如果设置，必须为 `https://...`。

#### account_key

!!! question "自 sing-box 1.14.0 起"

现有 ACME 帐户的 PEM 编码私钥。

#### trusted_roots_pem_files

!!! question "自 sing-box 1.14.0 起"

连接 ACME CA 时要额外信任的 PEM 文件列表。

适用于私有 ACME 部署或自定义测试 CA。

#### acme_timeout

!!! question "自 sing-box 1.14.0 起"

一次 ACME 申请或续期操作允许的最长时间。

#### profile

!!! question "自 sing-box 1.14.0 起"

要向 CA 请求的 ACME profile 名称。

并非所有 CA 都支持 ACME profile。

#### certificate_lifetime

!!! question "自 sing-box 1.14.0 起"

要向 CA 请求的证书有效期。

并非所有 CA 都支持自定义证书有效期。

#### disable_http_challenge

禁用所有 HTTP 质询。

#### disable_tls_alpn_challenge

禁用所有 TLS-ALPN 质询。

#### alternative_http_port

用于 ACME HTTP 质询的备用端口；如果非空，将使用此端口而不是 80 来启动 HTTP 质询的侦听器。

#### alternative_tls_port

用于 ACME TLS-ALPN 质询的备用端口； 系统必须将 443 转发到此端口以使质询成功。

#### bind_host

!!! question "自 sing-box 1.14.0 起"

启动 HTTP-01 或 TLS-ALPN 质询监听器时要绑定的主机地址。

#### external_account

EAB（外部帐户绑定）包含将 ACME 帐户绑定或映射到 CA 已知的其他帐户所需的信息。

外部帐户绑定用于将 ACME 帐户与非 ACME 系统中的现有帐户相关联，例如 CA 客户数据库。

为了启用 ACME 帐户绑定，运行 ACME 服务器的 CA 需要使用 ACME 之外的某种机制向 ACME 客户端提供 MAC 密钥和密钥标识符。§7.3.4

#### external_account.key_id

密钥标识符。

#### external_account.mac_key

MAC 密钥。

#### dns01_challenge

ACME DNS01 验证字段。如果配置，将禁用其他验证方法。

#### dns01_challenge.ttl

!!! question "自 sing-box 1.14.0 起"

DNS 验证临时 TXT 记录的 TTL。

#### dns01_challenge.propagation_delay

!!! question "自 sing-box 1.14.0 起"

创建验证记录后，在开始传播检查前要等待的时间。

#### dns01_challenge.propagation_timeout

!!! question "自 sing-box 1.14.0 起"

等待验证记录传播完成的最长时间。

设为 `-1` 可禁用传播检查。

#### dns01_challenge.resolvers

!!! question "自 sing-box 1.14.0 起"

进行 DNS 传播检查时优先使用的 DNS 解析器。

#### dns01_challenge.override_domain

!!! question "自 sing-box 1.14.0 起"

覆盖 DNS 验证记录使用的域名。

适用于将 `_acme-challenge` 委托到其他 zone 的场景。

提供商专有字段参阅 [DNS01 验证字段](/zh/configuration/shared/dns01_challenge/)。

#### preferred_chains

!!! question "自 sing-box 1.14.0 起"

用于选择 CA 提供的备用证书链的偏好设置。

必须至少设置 `smallest`、`root_common_name` 或 `any_common_name` 之一。

`root_common_name` 与 `any_common_name` 互斥。

#### preferred_chains.smallest

优先选择最小的证书链。

#### preferred_chains.root_common_name

优先选择根签发者通用名称匹配这些值之一的第一条证书链。

#### preferred_chains.any_common_name

优先选择签发者通用名称匹配这些值之一的第一条证书链。

#### key_type

!!! question "自 sing-box 1.14.0 起"

为新证书生成的私钥类型。

| 值         | 类型      |
|-----------|----------|
| `ed25519` | Ed25519 |
| `p256`    | P-256   |
| `p384`    | P-384   |
| `rsa2048` | RSA     |
| `rsa4096` | RSA     |

#### reuse_private_keys

!!! question "自 sing-box 1.14.0 起"

续期证书时复用存储中的现有私钥。

#### must_staple

!!! question "自 sing-box 1.14.0 起"

请求带有 TLS Must-Staple 扩展的证书。

#### renewal_window_ratio

!!! question "自 sing-box 1.14.0 起"

证书剩余多大比例生命周期时开始续期。

必须大于等于 `0` 且小于 `1`。

如果为空，则使用 certmagic 的默认续期窗口。

#### disable_ocsp_stapling

!!! question "自 sing-box 1.14.0 起"

禁用自动 OCSP stapling。

#### ocsp_overrides

!!! question "自 sing-box 1.14.0 起"

OCSP responder URL 到替换 URL 的映射。

将替换值设为空字符串可禁用对该 responder 的查询。
