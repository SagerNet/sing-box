---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

# Tailscale

### 结构

```json
{
  "type": "tailscale",
  "tag": "ts-cert",
  "endpoint": "ts-ep"
}
```

### 字段

#### endpoint

==必填==

要复用的 [Tailscale 端点](/zh/configuration/endpoint/tailscale/) 的标签。

必须在 Tailscale 管理控制台中启用 [MagicDNS 和 HTTPS](https://tailscale.com/kb/1153/enabling-https)。
