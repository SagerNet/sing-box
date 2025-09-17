---
icon: material/new-box
---

!!! question "自 sing-box 1.12.0 起"

# SSM API

SSM API 服务是一个用于管理 Shadowsocks 服务器的 RESTful API 服务器。

参阅 https://github.com/Shadowsocks-NET/shadowsocks-specs/blob/main/2023-1-shadowsocks-server-management-api-v1.md

### 结构

```json
{
  "type": "ssm-api",

  ... // 监听字段

  "servers": {},
  "cache_path": "",
  "tls": {}
}
```

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/) 了解详情。

### 字段

#### servers

==必填==

从 HTTP 端点到 [Shadowsocks 入站](/zh/configuration/inbound/shadowsocks) 标签的映射对象。

选定的 Shadowsocks 入站必须配置启用 [managed](/zh/configuration/inbound/shadowsocks#managed)。

示例：

```json
{
  "servers": {
    "/": "ss-in"
  }
}
```

#### cache_path

如果设置，当服务器即将停止时，流量和用户状态将保存到指定的 JSON 文件中，
以便在下次启动时恢复。

#### tls

TLS 配置，参阅 [TLS](/zh/configuration/shared/tls/#inbound)。