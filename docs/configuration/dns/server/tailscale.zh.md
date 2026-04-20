---
icon: material/new-box
---

!!! quote "sing-box 1.14.0 中的更改"

    :material-plus: [accept_search_domain](#accept_search_domain)

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
        "accept_default_resolvers": false,
        "accept_search_domain": false
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

#### accept_search_domain

!!! question "自 sing-box 1.14.0 起"

启用后，单标签查询（例如 `my-device`）将依次附加 Tailscale 搜索域进行重试，直到其中一个解析成功。

对于单标签查询，无论 `accept_default_resolvers` 是否启用，都不会使用默认 DNS 解析器。

### 示例

=== "仅 MagicDNS"

    === ":material-card-multiple: sing-box 1.14.0"

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
                "action": "evaluate",
                "server": "ts"
              },
              {
                "match_response": true,
                "ip_accept_any": true,
                "action": "respond"
              }
            ]
          }
        }
        ```

    === ":material-card-remove: sing-box < 1.14.0"

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