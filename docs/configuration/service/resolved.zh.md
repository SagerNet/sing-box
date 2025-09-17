---
icon: material/new-box
---

!!! question "自 sing-box 1.12.0 起"

# Resolved

Resolved 服务是一个伪造的 systemd-resolved DBUS 服务，用于从其他程序
（如 NetworkManager）接收 DNS 设置并提供 DNS 解析。

另请参阅：[Resolved DNS 服务器](/zh/configuration/dns/server/resolved/)

### 结构

```json
{
  "type": "resolved",

  ... // 监听字段
}
```

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/) 了解详情。

### 字段

#### listen

==必填==

监听地址。

默认使用 `127.0.0.53`。

#### listen_port

==必填==

监听端口。

默认使用 `53`。