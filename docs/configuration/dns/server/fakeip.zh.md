---
icon: material/new-box
---

!!! question "自 sing-box 1.12.0 起"

# Fake IP

### 结构

```json
{
  "dns": {
    "servers": [
      {
        "type": "fakeip",
        "tag": "",

        "inet4_range": "198.18.0.0/15",
        "inet6_range": "fc00::/18"
      }
    ]
  }
}
```

### 字段

#### inet4_range

FakeIP 的 IPv4 地址范围。

#### inet6_range

FakeIP 的 IPv6 地址范围。