---
icon: material/delete-clock
---

!!! failure "已在 sing-box 1.11.0 废弃"

    旧的特殊出站已被弃用，且将在 sing-box 1.13.0 中被移除, 参阅 [迁移指南](/migration/#migrate-legacy-special-outbounds-to-rule-actions). 

`dns` 出站是一个内部 DNS 服务器。

### 结构

```json
{
  "type": "dns",
  "tag": "dns-out"
}
```

!!! note ""

    DNS 出站没有出站连接，所有请求均在内部处理。

### 字段

无字段。