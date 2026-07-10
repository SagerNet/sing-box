---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

# Default

附加到已存在的网络命名空间。

### 结构

```json
{
  "network_namespaces": [
    {
      "type": "default", // 可选
      "tag": "",
      "path": ""
    }
  ]
}
```

### 字段

#### path

==必填==

网络命名空间的名称或路径，例如 `sing` 或 `/run/netns/sing`。
