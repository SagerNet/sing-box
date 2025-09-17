---
icon: material/new-box
---

!!! question "自 sing-box 1.12.0 起"

# DERP

DERP 服务是一个 Tailscale DERP 服务器，类似于 [derper](https://pkg.go.dev/tailscale.com/cmd/derper)。

### 结构

```json
{
  "type": "derp",

  ... // 监听字段

  "tls": {},
  "config_path": "",
  "verify_client_endpoint": [],
  "verify_client_url": [],
  "home": "",
  "mesh_with": [],
  "mesh_psk": "",
  "mesh_psk_file": "",
  "stun": {}
}
```

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/) 了解详情。

### 字段

#### tls

TLS 配置，参阅 [TLS](/zh/configuration/shared/tls/#inbound)。

#### config_path

==必填==

Derper 配置文件路径。

示例：`derper.key`

#### verify_client_endpoint

用于验证客户端的 Tailscale 端点标签。

#### verify_client_url

用于验证客户端的 URL。

对象格式：

```json
{
  "url": "https://my-headscale.com/verify",

  ... // 拨号字段
}
```

将数组值设置为字符串 `__URL__` 等同于配置：

```json
{ "url": __URL__ }
```

#### home

在根路径提供的内容。可以留空（默认值，显示默认主页）、`blank` 显示空白页面，或一个重定向的 URL。

#### mesh_with

与其他 DERP 服务器组网。

对象格式：

```json
{
  "server": "",
  "server_port": "",
  "host": "",
  "tls": {},

  ... // 拨号字段
}
```

对象字段：

- `server`：**必填** DERP 服务器地址。
- `server_port`：**必填** DERP 服务器端口。
- `host`：自定义 DERP 主机名。
- `tls`：[TLS](/zh/configuration/shared/tls/#outbound)
- `拨号字段`：[拨号字段](/zh/configuration/shared/dial/)

#### mesh_psk

DERP 组网的预共享密钥。

#### mesh_psk_file

DERP 组网的预共享密钥文件。

#### stun

STUN 服务器监听选项。

对象格式：

```json
{
  "enabled": true,

  ... // 监听字段
}
```

对象字段：

- `enabled`：**必填** 启用 STUN 服务器。
- `listen`：**必填** STUN 服务器监听地址，默认为 `::`。
- `listen_port`：**必填** STUN 服务器监听端口，默认为 `3478`。
- `其他监听字段`：[监听字段](/zh/configuration/shared/listen/)

将 `stun` 值设置为数字 `__PORT__` 等同于配置：

```json
{ "enabled": true, "listen_port": __PORT__ }
```