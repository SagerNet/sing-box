---
icon: material/new-box
---

!!! question "自 sing-box 1.13.0 起"

# CCM

CCM（Claude Code 多路复用器）服务是一个多路复用服务，允许您通过自定义令牌远程访问本地的 Claude Code 订阅。

它在本地机器上处理与 Claude API 的 OAuth 身份验证，同时允许远程 Claude Code 通过 `ANTHROPIC_AUTH_TOKEN` 环境变量使用认证令牌进行身份验证。

### 结构

```json
{
  "type": "ccm",

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

Claude Code OAuth 凭据文件的路径。

如果未指定，默认值为：
- 如果设置了 `CLAUDE_CONFIG_DIR` 环境变量，则使用 `$CLAUDE_CONFIG_DIR/.credentials.json`
- 否则使用 `~/.claude/.credentials.json`

在 macOS 上，首先从系统钥匙串读取凭据，如果不可用则回退到文件。

刷新的令牌会自动写回相同位置。

#### usages_path

用于存储聚合 API 使用统计信息的文件路径。

如果未指定，使用跟踪将被禁用。

启用后，服务会跟踪并保存全面的统计信息，包括：
- 请求计数
- 令牌使用量（输入、输出、缓存读取、缓存创建）
- 基于 Claude API 定价计算的美元成本

统计信息按模型、上下文窗口（200k 标准版 vs 1M 高级版）以及可选的用户（启用身份验证时）进行组织。

统计文件每分钟自动保存一次，并在服务关闭时保存。

#### users

用于令牌身份验证的授权用户列表。

如果为空，则不需要身份验证。

Claude Code 通过设置 `ANTHROPIC_AUTH_TOKEN` 环境变量为其令牌值进行身份验证。

#### headers

发送到 Claude API 的自定义 HTTP 头。

这些头会覆盖同名的现有头。

#### detour

用于连接 Claude API 的出站标签。

#### tls

TLS 配置，参阅 [TLS](/zh/configuration/shared/tls/#inbound)。

### 示例

```json
{
  "services": [
    {
      "type": "ccm",
      "listen": "127.0.0.1",
      "listen_port": 8080
    }
  ]
}
```

连接到 CCM 服务：

```bash
export ANTHROPIC_BASE_URL="http://127.0.0.1:8080"
export ANTHROPIC_AUTH_TOKEN="sk-ant-ccm-auth-token-not-required-in-this-context"

claude
```
