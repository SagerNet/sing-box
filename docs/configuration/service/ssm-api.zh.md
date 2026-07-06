---
icon: material/new-box
---

!!! quote "sing-box 1.14.0 中的更改"

    :material-alert-decagram: [servers](#servers)

!!! question "自 sing-box 1.12.0 起"

# SSM API

SSM API 服务是一个用于管理多用户入站的 RESTful API 服务器。

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

从 HTTP 端点到入站标签的映射对象。

支持的入站：

* [Shadowsocks](/zh/configuration/inbound/shadowsocks)（多用户）
* [HTTP](/zh/configuration/inbound/http)
* [Mixed](/zh/configuration/inbound/mixed)
* [SOCKS](/zh/configuration/inbound/socks)
* [Naive](/zh/configuration/inbound/naive)
* [Trojan](/zh/configuration/inbound/trojan)
* [VMess](/zh/configuration/inbound/vmess)
* [AnyTLS](/zh/configuration/inbound/anytls)
* [Hysteria](/zh/configuration/inbound/hysteria)
* [Hysteria2](/zh/configuration/inbound/hysteria2)

选定的 Shadowsocks 入站必须配置启用 [managed](/zh/configuration/inbound/shadowsocks#managed)。

由于 SSM API 用户对象仅包含单个凭据，对于 VMess 入站，用户密码即为 UUID，且受管理的用户始终使用 `alterId` `0`。

不支持 VLESS 和 TUIC 入站，因为它们的用户无法用单个凭据表示（分别为 `flow` 和 `uuid` + `password`）。

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

TLS 配置，参阅 [TLS](/zh/configuration/shared/tls/#入站)。