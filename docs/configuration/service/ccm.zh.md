---
icon: material/new-box
---

!!! question "自 sing-box 1.13.0 起"

# CCM

CCM（Claude Code 多路复用器）服务是一个代理服务器，允许使用 API 密钥身份验证代替 OAuth 来访问 Claude Code API。

它处理与 Claude API 的 OAuth 身份验证，并允许客户端通过 `x-api-key` 头使用简单的 API 密钥进行身份验证。

### 结构

```json
{
  "type": "ccm",

  ... // 监听字段

  "credential_path": "",
  "users": [],
  "headers": {},
  "detour": "",
  "tls": {}
}
```

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/) 了解详情。

### 字段

#### credential_path

Claude Code OAuth 凭据文件路径。

如果未指定，使用 `~/.claude/.credentials.json`。

在 macOS 上，首先从系统钥匙串读取凭据，然后回退到文件。

刷新的令牌会写回相同位置。

#### users

用于 API 密钥身份验证的用户列表。

如果为空，不执行身份验证。

客户端使用 `x-api-key` 头和令牌值进行身份验证。

#### headers

发送到 Claude API 的自定义 HTTP 头。

这些头会覆盖同名的现有头。

#### detour

用于连接到 Claude API 的出站标签。

#### tls

TLS 配置，参阅 [TLS](/zh/configuration/shared/tls/#inbound)。
