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

  "hostnames": [],
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

#### hostnames

要写入证书的主机名或通配符主机名列表。

Unicode 主机名会自动转换为 punycode。

#### data_directory

保存签发证书和私钥的目录。

如果为空，证书只保存在内存中，重启后会重新签发。

#### api_token

用于创建证书的 Cloudflare API Token。

需要 `Zone / SSL and Certificates / Edit` 权限。

与 `origin_ca_key` 互斥。

#### origin_ca_key

作为 `X-Auth-User-Service-Key` 请求头发送的 Cloudflare Origin CA Key。

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
