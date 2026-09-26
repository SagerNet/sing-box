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

    [Local](./local/) 服务器也会解析 `*.local.` 与 IPv4/IPv6 链路本地反向区域。

### 字段

#### interface

用于发送 mDNS 查询的网络接口名称列表。

留空时，将使用所有处于 up 状态、支持多播且非环回的接口。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/) 了解详情。
