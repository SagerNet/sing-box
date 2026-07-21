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

DNS 查询会通过该端点发送到 OpenVPN 服务器推送的解析器。现代 OpenVPN `dns server` 选项支持普通 DNS、DNS over TLS、DNS over HTTPS、自定义端口、SNI 和 `resolve-domains`。只有优先级数字最低的服务器组会生效。没有现代服务器组时，使用传统的 `dhcp-option DNS`/`DNS6` 和 `DOMAIN-ROUTE`。

现代服务器组会覆盖传统 DHCP DNS 解析器及相关域选项。只有现代 `dns search-domains` 而没有现代服务器组时，不会移除传统解析器。由于此传输不提供 DNSSEC 验证，需要强制验证的 `dnssec yes` 会被拒绝。

推送的 DNS 设置不会安装到操作系统中。

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
