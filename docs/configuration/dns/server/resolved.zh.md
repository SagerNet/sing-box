---
icon: material/new-box
---

!!! question "自 sing-box 1.12.0 起"

# Resolved

```json
{
  "dns": {
    "servers": [
      {
        "type": "resolved",
        "tag": "",

        "service": "resolved",
        "accept_default_resolvers": false
      }
    ]
  }
}
```

### 字段

#### service

==必填==

[Resolved 服务](/zh/configuration/service/resolved) 的标签。

#### accept_default_resolvers

指示是否除了匹配域名外，还应接受默认 DNS 解析器以进行回退查询。

具体来说，默认 DNS 解析器是设置了 `SetLinkDefaultRoute` 或 `SetLinkDomains ~.` 的 DNS 服务器。

如果未启用，对于不匹配搜索域或匹配域的请求，将返回 `NXDOMAIN`。

### 示例

=== "仅分割 DNS"

    ```json
    {
      "dns": {
        "servers": [
          {
            "type": "local",
            "tag": "local"
          },
          {
            "type": "resolved",
            "tag": "resolved",
            "service": "resolved"
          }
        ],
        "rules": [
          {
            "ip_accept_any": true,
            "server": "resolved"
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
            "type": "resolved",
            "service": "resolved",
            "accept_default_resolvers": true
          }
        ]
      }
    }
    ```