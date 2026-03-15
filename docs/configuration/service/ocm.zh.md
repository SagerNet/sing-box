---
icon: material/new-box
---

!!! question "自 sing-box 1.13.0 起"

# OCM

OCM（OpenAI Codex 多路复用器）服务是一个多路复用服务，允许您通过自定义令牌远程访问本地的 OpenAI Codex 订阅。

它在本地机器上处理与 OpenAI API 的 OAuth 身份验证，同时允许远程客户端使用自定义令牌进行身份验证。

!!! quote "sing-box 1.14.0 中的更改"

    :material-plus: [credentials](#credentials)  
    :material-alert: [credential_path](#credential_path)  
    :material-alert: [usages_path](#usages_path)  
    :material-alert: [users](#users)  
    :material-alert: [detour](#detour)

### 结构

```json
{
  "type": "ocm",

  ... // 监听字段

  "credential_path": "",
  "credentials": [],
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

如果未指定，默认值为：
- 如果设置了 `CODEX_HOME` 环境变量，则使用 `$CODEX_HOME/auth.json`
- 否则使用 `~/.codex/auth.json`

刷新的令牌会自动写回相同位置。

!!! question "自 sing-box 1.14.0 起"

当 `credential_path` 指向文件时，即使文件尚不存在，服务也可以启动。文件被创建或更新后，凭据会自动变为可用；如果文件之后被删除或变为无效，该凭据会立即变为不可用。

与 `credentials` 冲突。

#### credentials

!!! question "自 sing-box 1.14.0 起"

多凭据模式的凭据配置列表。

设置后，顶层 `credential_path`、`usages_path` 和 `detour` 被禁止。每个用户必须指定 `credential` 标签。

每个凭据有一个 `type` 字段（`default`、`external` 或 `balancer`）和一个必填的 `tag` 字段。

##### 默认凭据

```json
{
  "tag": "a",
  "credential_path": "/path/to/auth.json",
  "usages_path": "/path/to/usages.json",
  "detour": "",
  "reserve_5h": 20,
  "reserve_weekly": 20,
  "limit_5h": 0,
  "limit_weekly": 0
}
```

单个 OAuth 凭据文件。`type` 字段可以省略（默认为 `default`）。即使文件尚不存在，服务也可以启动，并会自动重载文件更新。

- `credential_path`：凭据文件的路径。默认值与顶层 `credential_path` 相同。
- `usages_path`：此凭据的可选使用跟踪文件。
- `detour`：此凭据用于连接 OpenAI API 的出站标签。
- `reserve_5h`：主要速率限制窗口的保留阈值（1-99）。凭据在利用率达到 (100-N)% 时暂停。与 `limit_5h` 冲突。
- `reserve_weekly`：次要（每周）速率限制窗口的保留阈值（1-99）。凭据在利用率达到 (100-N)% 时暂停。与 `limit_weekly` 冲突。
- `limit_5h`：主要速率限制窗口的显式利用率上限（0-100）。`0` 表示未设置显式上限。凭据在利用率达到此值时暂停。与 `reserve_5h` 冲突。
- `limit_weekly`：次要（每周）速率限制窗口的显式利用率上限（0-100）。`0` 表示未设置显式上限。凭据在利用率达到此值时暂停。与 `reserve_weekly` 冲突。

##### 均衡凭据

```json
{
  "tag": "pool",
  "type": "balancer",
  "strategy": "",
  "credentials": ["a", "b"],
  "poll_interval": "60s"
}
```

根据选择的策略将会话分配给默认凭据。会话保持粘性，直到分配的凭据触发速率限制。

- `strategy`：选择策略。可选值：`least_used` `round_robin` `random` `fallback`。默认使用 `least_used`。
- `credentials`：==必填== 默认凭据标签列表。
- `poll_interval`：轮询上游使用 API 的间隔。默认 `60s`。

##### 回退策略

```json
{
  "tag": "backup",
  "type": "balancer",
  "strategy": "fallback",
  "credentials": ["a", "b"],
  "poll_interval": "30s"
}
```

将 `strategy` 设为 `fallback` 的均衡凭据会按顺序使用凭据。当前凭据耗尽后切换到下一个。

- `credentials`：==必填== 有序的默认凭据标签列表。
- `poll_interval`：轮询上游使用 API 的间隔。默认 `60s`。

##### 外部凭据

```json
{
  "tag": "remote",
  "type": "external",
  "url": "",
  "server": "",
  "server_port": 0,
  "token": "",
  "reverse": false,
  "detour": "",
  "usages_path": "",
  "poll_interval": "30m"
}
```

通过远程 OCM 实例代理请求，而非使用本地 OAuth 凭据。

- `url`：远程 OCM 实例的 URL。省略时，此凭据作为仅等待入站反向连接的接收器。
- `server`：覆盖拨号的服务器地址，与 URL 主机名分开。
- `server_port`：覆盖拨号的服务器端口。
- `token`：==必填== 远程实例的身份验证令牌。
- `reverse`：启用连接器模式。要求设置 `url`。启用后，此凭据会主动拨出到远程实例的 `/ocm/v1/reverse`，且不能直接为本地请求提供服务。当设置了 `url` 但未启用 `reverse` 时，此凭据会正常通过远程实例转发请求，并在反向连接建立后优先使用该反向连接。
- `detour`：用于连接远程实例的出站标签。
- `usages_path`：可选的使用跟踪文件。
- `poll_interval`：轮询远程状态端点的间隔。默认 `30m`。

#### usages_path

用于存储聚合 API 使用统计信息的文件路径。

如果未指定，使用跟踪将被禁用。

启用后，服务会跟踪并保存全面的统计信息，包括：
- 请求计数
- 令牌使用量（输入、输出、缓存）
- 基于 OpenAI API 定价计算的美元成本

统计信息按模型以及可选的用户（启用身份验证时）进行组织。

统计文件每分钟自动保存一次，并在服务关闭时保存。

!!! question "自 sing-box 1.14.0 起"

与 `credentials` 冲突。在多凭据模式下，在各个默认凭据上使用 `usages_path`。

#### users

用于令牌身份验证的授权用户列表。

如果为空，则不需要身份验证。

对象格式：

```json
{
  "name": "",
  "token": "",
  "credential": "",
  "external_credential": "",
  "allow_external_usage": false
}
```

对象字段：

- `name`：用于跟踪的用户名标识符。
- `token`：用于身份验证的 Bearer 令牌。客户端通过设置 `Authorization: Bearer <token>` 头进行身份验证。

!!! question "自 sing-box 1.14.0 起"

- `credential`：此用户使用的凭据标签。设置 `credentials` 时==必填==。
- `external_credential`：仅用于用此用户其他可用凭据的聚合利用率重写响应速率限制头的外部凭据标签。它不参与请求路由；请求选择仍由 `credential` 和 `allow_external_usage` 决定。
- `allow_external_usage`：允许此用户使用外部凭据。默认为 `false`。

#### headers

发送到 OpenAI API 的自定义 HTTP 头。

这些头会覆盖同名的现有头。

#### detour

用于连接 OpenAI API 的出站标签。

!!! question "自 sing-box 1.14.0 起"

与 `credentials` 冲突。在多凭据模式下，在各个默认凭据上使用 `detour`。

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
# profile = "ocm"                # 设为默认配置


[model_providers.ocm]
name = "OCM Proxy"
base_url = "http://127.0.0.1:8080/v1"
supports_websockets = true

[profiles.ocm]
model_provider = "ocm"
# model = "gpt-5.4"              # 如果最新模型尚未公开发布
# model_reasoning_effort = "xhigh"
```

然后运行：

```bash
codex --profile ocm
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
          "token": "sk-ocm-hello-world"
        },
        {
          "name": "bob",
          "token": "sk-ocm-hello-bob"
        }
      ]
    }
  ]
}
```

#### 客户端

在 `~/.codex/config.toml` 中添加：

```toml
# profile = "ocm"                # 设为默认配置

[model_providers.ocm]
name = "OCM Proxy"
base_url = "http://127.0.0.1:8080/v1"
supports_websockets = true
experimental_bearer_token = "sk-ocm-hello-world"

[profiles.ocm]
model_provider = "ocm"
# model = "gpt-5.4"              # 如果最新模型尚未公开发布
# model_reasoning_effort = "xhigh"
```

然后运行：

```bash
codex --profile ocm
```

### 多凭据示例

#### 服务端

```json
{
  "services": [
    {
      "type": "ocm",
      "listen": "0.0.0.0",
      "listen_port": 8080,
      "credentials": [
        {
          "tag": "a",
          "credential_path": "/home/user/.codex-a/auth.json",
          "usages_path": "/data/usages-a.json",
          "reserve_5h": 20,
          "reserve_weekly": 20
        },
        {
          "tag": "b",
          "credential_path": "/home/user/.codex-b/auth.json",
          "reserve_5h": 10,
          "reserve_weekly": 10
        },
        {
          "tag": "pool",
          "type": "balancer",
          "poll_interval": "60s",
          "credentials": ["a", "b"]
        }
      ],
      "users": [
        {
          "name": "alice",
          "token": "sk-ocm-hello-world",
          "credential": "pool"
        },
        {
          "name": "bob",
          "token": "sk-ocm-hello-bob",
          "credential": "a"
        }
      ]
    }
  ]
}
```
