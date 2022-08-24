# DNS

### 结构

```json
{
  "dns": {
    "servers": [],
    "rules": [],
    "final": "",
    "strategy": "",
    "disable_cache": false,
    "disable_expire": false
  }
}

```

### 字段

| 键        | 格式                     |
|----------|------------------------|
| `server` | 一组 [DNS 服务器](./server) |
| `rules`  | 一组 [DNS 规则](./rule)    |

#### final

默认 DNS 服务器的标签。

将使用第一个服务器，如果为空。

#### strategy

默认解析域名策略。

可选值: `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`。

如果设置了 `server.strategy`，则不生效。

#### disable_cache

禁用 DNS 缓存。

#### disable_expire

禁用 DNS 缓存过期。