---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

# 证书提供者

### 结构

```json
{
  "certificate_providers": [
    {
      "type": "",
      "tag": ""
    }
  ]
}
```

### 字段

| 类型   | 格式             |
|--------|------------------|
| `acme` | [ACME](/zh/configuration/shared/certificate-provider/acme)   |
| `tailscale` | [Tailscale](/zh/configuration/shared/certificate-provider/tailscale) |
| `cloudflare-origin-ca` | [Cloudflare Origin CA](/zh/configuration/shared/certificate-provider/cloudflare-origin-ca) |

#### tag

证书提供者的标签。
