---
icon: material/note-remove
---

!!! failure "已在 sing-box 1.14.0 移除"

    旧的 fake-ip 配置已在 sing-box 1.12.0 废弃且已在 sing-box 1.14.0 中被移除，参阅 [迁移指南](/zh/migration/#迁移到新的-dns-服务器格式)。

### 结构

```json
{
  "enabled": true,
  "inet4_range": "198.18.0.0/15",
  "inet6_range": "fc00::/18"
}
```

### 字段

#### enabled

启用 FakeIP 服务。

#### inet4_range

用于 FakeIP 的 IPv4 地址范围。

#### inet6_range

用于 FakeIP 的 IPv6 地址范围。
