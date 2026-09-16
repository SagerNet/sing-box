---
icon: material/new-box
---

!!! question "自 sing-box 1.15.0 起"

### 结构

```json
{
  "type": "tailcat",
  "tag": "tailcat-out",

  "private_key": "",
  "server_public_key": "",
  "server_disco_key": "",
  "pre_shared_key": "",
  "derp_map_url": "",
  "derp_region": 0,
  "derp_servers": [],
  "http_client": {},

  ... // 拨号字段
}
```

### 字段

#### private_key

私钥。

默认使用随机密钥。

当服务器通过 `users` 验证客户端，或 DERP 服务器通过 `verify_client_inbound` 或 `verify_client_key`
验证客户端时必须设置。

#### server_public_key

==必填==

服务器公钥。

#### server_disco_key

==必填==

服务器 disco 公钥。

#### pre_shared_key

预共享密钥。

#### derp_map_url

[DERP map](https://pkg.go.dev/tailscale.com/tailcfg#DERPMap) 的 URL。

默认使用 `https://tailcat.dev/derpmap.json`。

#### derp_region

DERP map 中的 DERP 区域 ID。

与 `derp_servers` 冲突。

#### derp_servers

自定义 DERP 服务器，参阅 Tailcat 入站的 [derp_servers](/zh/configuration/inbound/tailcat/#derp_servers)。

与 `derp_map_url` 和 `derp_region` 冲突。

#### http_client

用于获取 DERP map 的 HTTP 客户端。

参阅 [HTTP 客户端](/zh/configuration/shared/http-client/) 了解详情。

### 拨号字段

!!! note

    Tailcat 出站中的拨号字段仅控制它如何连接到 DERP 服务器，与实际连接无关。

参阅 [拨号字段](/zh/configuration/shared/dial/) 了解详情。
