---
icon: material/new-box
---

!!! question "自 sing-box 1.13.0 起"

# OCM

OCM（OpenAI Codex 多路复用器）服务是一个多路复用服务，允许您通过自定义令牌远程访问本地的 OpenAI Codex 订阅。

它在本地机器上处理与 OpenAI API 的 OAuth 身份验证，同时允许远程客户端使用自定义令牌进行身份验证。

### 结构

```json
{
  "type": "ocm",

  ... // 监听字段

  "credential_path": "",
  "usages_path": "",
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

OpenAI OAuth 凭据文件的路径。

如果未指定，默认值为 `~/.codex/auth.json`。

刷新的令牌会自动写回相同位置。

#### usages_path

用于存储聚合 API 使用统计信息的文件路径。

如果未指定，使用跟踪将被禁用。

启用后，服务会跟踪并保存全面的统计信息，包括：
- 请求计数
- 令牌使用量（输入、输出、缓存）
- 基于 OpenAI API 定价计算的美元成本

统计信息按模型以及可选的用户（启用身份验证时）进行组织。

统计文件每分钟自动保存一次，并在服务关闭时保存。

#### users

用于令牌身份验证的授权用户列表。

如果为空，则不需要身份验证。

对象格式：

```json
{
  "name": "",
  "token": ""
}
```

对象字段：

- `name`：用于跟踪的用户名标识符。
- `token`：用于身份验证的 Bearer 令牌。客户端通过设置 `Authorization: Bearer <token>` 头进行身份验证。

#### headers

发送到 OpenAI API 的自定义 HTTP 头。

这些头会覆盖同名的现有头。

#### detour

用于连接 OpenAI API 的出站标签。

#### tls

TLS 配置，参阅 [TLS](/zh/configuration/shared/tls/#inbound)。

### 示例

#### 服务端

```json
{
  "services": [
    {
      "type": "ocm",
      "listen": "127.0.0.1",
      "listen_port": 8080
    }
  ]
}
```

#### 客户端

在 `~/.codex/config.toml` 中添加：

```toml
[model_providers.ocm]
name = "OCM Proxy"
base_url = "http://127.0.0.1:8080/v1"
wire_api = "responses"
requires_openai_auth = false
```

然后运行：

```bash
codex --model-provider ocm
```

### 带身份验证的示例

#### 服务端

```json
{
  "services": [
    {
      "type": "ocm",
      "listen": "0.0.0.0",
      "listen_port": 8080,
      "usages_path": "./codex-usages.json",
      "users": [
        {
          "name": "alice",
          "token": "sk-alice-secret-token"
        },
        {
          "name": "bob",
          "token": "sk-bob-secret-token"
        }
      ]
    }
  ]
}
```

#### 客户端

在 `~/.codex/config.toml` 中添加：

```toml
[model_providers.ocm]
name = "OCM Proxy"
base_url = "http://127.0.0.1:8080/v1"
wire_api = "responses"
requires_openai_auth = false
experimental_bearer_token = "sk-alice-secret-token"
```

然后运行：

```bash
codex --model-provider ocm
```
