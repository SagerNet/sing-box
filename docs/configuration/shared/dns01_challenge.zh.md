---
icon: material/new-box
---

!!! quote "sing-box 1.13.0 中的更改"

    :material-plus: [alidns.security_token](#security_token)  
    :material-plus: [cloudflare.zone_token](#zone_token)

### 结构

```json
{
  "provider": "",

  ... // 提供商字段
}
```

### 提供商字段

#### Alibaba Cloud DNS

```json
{
  "provider": "alidns",
  "access_key_id": "",
  "access_key_secret": "",
  "region_id": "",
  "security_token": ""
}
```

##### security_token

!!! question "自 sing-box 1.13.0 起"

用于 STS 临时凭证的安全令牌。

#### Cloudflare

```json
{
  "provider": "cloudflare",
  "api_token": "",
  "zone_token": ""
}
```

##### zone_token

!!! question "自 sing-box 1.13.0 起"

具有 `Zone:Read` 权限的可选 API 令牌。

提供后可将 `api_token` 限定到单个区域。
