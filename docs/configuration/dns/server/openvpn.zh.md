---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

# OpenVPN

### 结构

```json
{
  "dns": {
    "servers": [
      {
        "type": "openvpn",
        "tag": "",

        "endpoint": "ovpn-client",
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

[OpenVPN 客户端端点](/zh/configuration/endpoint/openvpn-client) 的标签。

DNS 查询会通过该端点发送到 OpenVPN 服务器推送的解析器。

支持现代 `dns` 选项，以及传统的 `dhcp-option DNS`/`DNS6` 和 `DOMAIN-ROUTE` 选项。不支持 `dnssec yes`。

#### accept_default_resolvers

对未匹配推送的 `resolve-domains`、`DOMAIN-ROUTE` 或搜索域后缀的查询使用推送解析器。

禁用时，未匹配查询返回 `NXDOMAIN`。

#### accept_search_domain

启用且存在推送的搜索域时，单标签查询（例如 `intranet`）会依次附加各个搜索域重试，直到其中一个解析成功。

不存在搜索域时，原始单标签查询按普通默认解析器规则处理。

### 示例

```json
{
  "dns": {
    "servers": [
      {
        "type": "local",
        "tag": "local"
      },
      {
        "type": "openvpn",
        "tag": "ovpn-dns",
        "endpoint": "ovpn-client",
        "accept_default_resolvers": true,
        "accept_search_domain": true
      }
    ],
    "rules": [
      {
        "preferred_by": "ovpn-dns",
        "action": "route",
        "server": "ovpn-dns"
      }
    ],
    "final": "local"
  }
}
```
