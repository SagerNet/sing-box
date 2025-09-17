---
icon: material/new-box
---

!!! question "自 sing-box 1.12.0 起"

# Tailscale

### 结构

```json
{
  "dns": {
    "servers": [
      {
        "type": "tailscale",
        "tag": "",

        "endpoint": "ts-ep",
        "accept_default_resolvers": false
      }
    ]
  }
}
```

### 字段

#### endpoint

==必填==

[Tailscale 端点](/zh/configuration/endpoint/tailscale) 的标签。

#### accept_default_resolvers

指示是否除了 MagicDNS 外，还应接受默认 DNS 解析器以进行回退查询。

如果未启用，对于非 Tailscale 域名查询将返回 `NXDOMAIN`。

### 示例

=== "仅 MagicDNS"

    ```json
    {
      "dns": {
        "servers": [
          {
            "type": "local",
            "tag": "local"
          },
          {
            "type": "tailscale",
            "tag": "ts",
            "endpoint": "ts-ep"
          }
        ],
        "rules": [
          {
            "ip_accept_any": true,
            "server": "ts"
          }
        ]
      }
    }
    ```

=== "用作全局 DNS"

    ```json
    {
      "dns": {
        "servers": [
          {
            "type": "tailscale",
            "endpoint": "ts-ep",
            "accept_default_resolvers": true
          }
        ]
      }
    }
    ```