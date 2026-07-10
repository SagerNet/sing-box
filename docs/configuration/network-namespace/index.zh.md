---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

!!! quote ""

    仅支持 Linux。

# 网络命名空间

网络命名空间使入站和出站可以运行在独立的 Linux 网络命名空间中，
通过标签从 [tun](/zh/configuration/inbound/tun/#netns)、
[监听字段](/zh/configuration/shared/listen/#netns) 和 [拨号字段](/zh/configuration/shared/dial/#netns) 引用。

### 结构

```json
{
  "network_namespaces": [
    {
      "type": "",
      "tag": ""
    }
  ]
}
```

#### type

网络命名空间的类型，默认使用 `default`。

| 类型      | 格式                   |
|-----------|------------------------|
| `default` | [Default](./default/)  |
| `unshare` | [Unshare](./unshare/)  |

#### tag

==必填==

网络命名空间的标签。
