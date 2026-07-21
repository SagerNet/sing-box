---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

# OpenConnect

### 结构

```json
{
  "dns": {
    "servers": [
      {
        "type": "openconnect",
        "tag": "",

        "endpoint": "oc-client",
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

[OpenConnect 端点](/zh/configuration/endpoint/openconnect) 的标签。

DNS 查询会通过 OpenConnect 端点发送到 VPN 服务器推送的解析器。推送的分流 DNS 规则使用各自的专用解析器，推送的分流 DNS 和搜索域后缀则使用通用推送解析器。匹配时优先使用最具体的后缀。

推送的 DNS 设置不会安装到操作系统中。

#### accept_default_resolvers

接受 VPN 服务器推送的通用解析器，用于未匹配的查询。

启用时，仅当服务器要求所有 DNS 通过隧道，或未提供分流 DNS 规则及后缀时，通用解析器才会作为默认解析器。否则，未匹配的查询将返回 `NXDOMAIN`。

#### accept_search_domain

启用且存在推送的搜索域时，单标签查询（例如 `intranet`）会依次附加各个搜索域进行重试，直到其中一个解析成功。

如果所有搜索域扩展均返回 `NXDOMAIN`，原始未限定名称将按普通默认解析器行为处理。

### 示例

=== "仅分流 DNS"

    ```json
    {
      "dns": {
        "servers": [
          {
            "type": "local",
            "tag": "local"
          },
          {
            "type": "openconnect",
            "tag": "oc",
            "endpoint": "oc-client"
          }
        ],
        "rules": [
          {
            "preferred_by": "oc",
            "action": "route",
            "server": "oc"
          }
        ],
        "final": "local"
      }
    }
    ```

=== "接受推送的默认解析器"

    ```json
    {
      "dns": {
        "servers": [
          {
            "type": "openconnect",
            "endpoint": "oc-client",
            "accept_default_resolvers": true,
            "accept_search_domain": true
          }
        ]
      }
    }
    ```
