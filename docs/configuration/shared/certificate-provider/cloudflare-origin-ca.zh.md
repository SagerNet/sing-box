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
  "http_client": "" // 或 {}
}
```

### 字段

#### domain

==必填==

要写入证书的域名或通配符域名列表。

#### data_directory

保存签发证书、私钥和元数据的根目录。

如果为空，sing-box 会使用与 ACME 证书提供者相同的默认数据目录：
`$XDG_DATA_HOME/certmagic` 或 `$HOME/.local/share/certmagic`。

#### api_token

用于创建证书的 Cloudflare API Token。

可在 [Cloudflare Dashboard > My Profile > API Tokens](https://dash.cloudflare.com/profile/api-tokens) 获取或创建。

需要 `Zone / SSL and Certificates / Edit` 权限。

与 `origin_ca_key` 冲突。

#### origin_ca_key

Cloudflare Origin CA Key。

可在 [Cloudflare Dashboard > My Profile > API Tokens > API Keys > Origin CA Key](https://dash.cloudflare.com/profile/api-tokens) 获取。

与 `api_token` 冲突。

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

如果为空，使用 `5475` 天（15 年）。

#### http_client

用于所有提供者 HTTP 请求的 HTTP 客户端。

参阅 [HTTP 客户端字段](/zh/configuration/shared/http-client/) 了解详情。
