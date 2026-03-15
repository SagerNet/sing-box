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

要复用的 [Tailscale 端点](/configuration/endpoint/tailscale/) 的标签。

该证书提供者会复用端点内部的 `tsnet` 节点，并通过 Tailscale 的本地 API
获取证书，行为类似 `tsnet.ListenTLS`。

被引用的端点必须已经连接到目标 tailnet，并且需要在 Tailscale 管理面板中
启用 MagicDNS 和 HTTPS，证书签发才能成功。
