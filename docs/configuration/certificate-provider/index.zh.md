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

#### tag

证书提供者的标签。
