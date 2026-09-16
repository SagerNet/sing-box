---
icon: material/new-box
---

!!! question "自 sing-box 1.15.0 起"

### 结构

```json
{
  "type": "tailcat",
  "tag": "tailcat-in",

  "private_key": "",
  "pre_shared_key": "",
  "users": [
    {
      "name": "",
      "public_key": ""
    }
  ],
  "derp_map_url": "",
  "derp_region": 0,
  "derp_servers": [],
  "http_client": {},

  ... // 拨号字段
}
```

### 字段

#### private_key

==必填==

私钥。

使用 `sing-box generate tailcat-keypair` 生成。

#### pre_shared_key

预共享密钥。

#### users

Tailcat 用户。

为空时不验证客户端。

如需使用带客户端验证的 DERP 服务器，在 [DERP 服务](/zh/configuration/service/derp/#verify_client_inbound)
中设置 `verify_client_inbound` 或 `verify_client_key`，且客户端必须使用固定的私钥。

#### users.public_key

==必填==

客户端公钥。

#### derp_map_url

[DERP map](https://pkg.go.dev/tailscale.com/tailcfg#DERPMap) 的 URL。

默认使用 `https://tailcat.dev/derpmap.json`。

#### derp_region

DERP map 中的 DERP 区域 ID。

与 `derp_servers` 冲突。

#### derp_servers

自定义 DERP 服务器，为 [DERPNode](https://pkg.go.dev/tailscale.com/tailcfg#DERPNode) 格式，字段名使用 snake_case。

与 `derp_map_url` 和 `derp_region` 冲突。

将数组值设置为字符串 `__HOST__` 等同于配置：

```json
{ "host": __HOST__ }
```

#### http_client

用于获取 DERP map 的 HTTP 客户端。

参阅 [HTTP 客户端](/zh/configuration/shared/http-client/) 了解详情。

### 拨号字段

!!! note

    Tailcat 入站中的拨号字段仅控制它如何连接到 DERP 服务器，与实际连接无关。

参阅 [拨号字段](/zh/configuration/shared/dial/) 了解详情。
