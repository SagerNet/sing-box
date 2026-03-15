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
| `acme` | [ACME](./acme)   |
| `tailscale` | [Tailscale](./tailscale) |
| `cloudflare-origin-ca` | [Cloudflare Origin CA](./cloudflare-origin-ca) |

`acme` 类型用于迁移已废弃的内联 `tls.acme` 选项。
sing-box 1.14.0 新增字段参阅 [ACME](./acme) 页面。

#### tag

证书提供者的标签。
