---
icon: material/new-box
---

!!! quote "sing-box 1.14.0 中的更改"

    :material-plus: [ttl](#ttl)  
    :material-plus: [propagation_delay](#propagation_delay)  
    :material-plus: [propagation_timeout](#propagation_timeout)  
    :material-plus: [resolvers](#resolvers)  
    :material-plus: [override_domain](#override_domain)

!!! quote "sing-box 1.13.0 中的更改"

    :material-plus: [alidns.security_token](#security_token)  
    :material-plus: [cloudflare.zone_token](#zone_token)  
    :material-plus: [acmedns](#acmedns)

### 结构

```json
{
  "ttl": "",
  "propagation_delay": "",
  "propagation_timeout": "",
  "resolvers": [],
  "override_domain": "",
  "provider": "",

  ... // 提供商字段
}
```

### 字段

#### ttl

!!! question "自 sing-box 1.14.0 起"

DNS 质询临时 TXT 记录的 TTL。

#### propagation_delay

!!! question "自 sing-box 1.14.0 起"

创建质询记录后，在开始传播检查前要等待的时间。

#### propagation_timeout

!!! question "自 sing-box 1.14.0 起"

等待质询记录传播完成的最长时间。

设为 `-1` 可禁用传播检查。

#### resolvers

!!! question "自 sing-box 1.14.0 起"

进行 DNS 传播检查时优先使用的 DNS 解析器。

#### override_domain

!!! question "自 sing-box 1.14.0 起"

覆盖 DNS 质询记录使用的域名。

适用于将 `_acme-challenge` 委托到其他 zone 的场景。

#### provider

DNS 提供商。提供商专有字段见下文。

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

#### ACME-DNS

!!! question "自 sing-box 1.13.0 起"

```json
{
  "provider": "acmedns",
  "username": "",
  "password": "",
  "subdomain": "",
  "server_url": ""
}
```

参阅 [ACME-DNS](https://github.com/joohoi/acme-dns)。
