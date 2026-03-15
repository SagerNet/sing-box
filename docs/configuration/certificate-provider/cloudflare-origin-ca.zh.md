---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

# Cloudflare Origin CA

### 结构

```json
{
  "type": "cloudflare-origin-ca",
  "tag": "",

  "domain": [],
  "data_directory": "",
  "api_token": "",
  "origin_ca_key": "",
  "request_type": "",
  "requested_validity": 0,
  "renew_before": "",
  "request_timeout": ""
}
```

该提供者会在本地生成私钥，向 Cloudflare Origin CA 提交 CSR，并将返回的证书作为 TLS 证书提供者使用。

### 字段

#### domain

要写入证书的域名或通配符域名列表。

#### data_directory

保存签发证书、私钥和元数据的根目录。

文件会按 CertMagic 的证书存储布局写入。

如果为空，sing-box 会使用与 ACME 证书提供者相同的 CertMagic 默认数据目录：
`$XDG_DATA_HOME/certmagic` 或 `$HOME/.local/share/certmagic`。

#### api_token

用于创建证书的 Cloudflare API Token。

可在 [Cloudflare Dashboard > My Profile > API Tokens](https://dash.cloudflare.com/profile/api-tokens) 获取或创建。

需要 `Zone / SSL and Certificates / Edit` 权限。

与 `origin_ca_key` 互斥。

#### origin_ca_key

作为 `X-Auth-User-Service-Key` 请求头发送的 Cloudflare Origin CA Key。

可在 [Cloudflare Dashboard > My Profile > API Tokens > API Keys > Origin CA Key](https://dash.cloudflare.com/profile/api-tokens) 获取。

与 `api_token` 互斥。

#### request_type

向 Cloudflare 请求的签名类型。

| 值                   | 类型        |
|----------------------|-------------|
| `origin-rsa`         | RSA         |
| `origin-ecc`         | ECDSA P-256 |

如果为空，使用 `origin-rsa`。

#### requested_validity

请求的证书有效期，单位为天。

可用值：`7`、`30`、`90`、`365`、`730`、`1095`、`5475`。

如果为空，使用 `5475`。

#### renew_before

sing-box 会在证书过期前多长时间开始请求新的证书。

如果为空，使用 `30d` 与证书生命周期三分之一中的较小值。

#### request_timeout

访问 Cloudflare API 的 HTTP 超时时间。

如果为空，使用 `30s`。
