---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

# mDNS

### 结构

```json
{
  "dns": {
    "servers": [
      {
        "type": "mdns",
        "tag": "",

        "interface": [],

        // 拨号字段
      }
    ]
  }
}
```

!!! info ""

    通常不需要在 [Local](./local/) 服务器之外再添加显式的 `mdns` 服务器：本地服务器已经会在非 Apple 平台通过 mDNS、在 Apple 平台通过系统解析器来回答 `*.local.` 与 IPv4/IPv6 链路本地反向区域的查询。仅当需要从 [`preferred_by`](../rule/#preferred_by) 引用，或独立使用时，才需要显式添加 `mdns` 服务器。

### 字段

#### interface

用于发送 mDNS 查询的网络接口名称列表。

留空时，将使用所有处于 up 状态、支持多播且非环回的接口。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/) 了解详情。
