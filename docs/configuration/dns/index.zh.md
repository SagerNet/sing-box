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
    "disable_expire": false,
    "reverse_mapping": false
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

默认使用第一个服务器。

#### strategy

默认解析域名策略。

可选值: `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`。

如果设置了 `server.strategy`，则不生效。

#### disable_cache

禁用 DNS 缓存。

#### disable_expire

禁用 DNS 缓存过期。

#### reverse_mapping

在响应 DNS 查询后存储 IP 地址的反向映射以为路由目的提供域名。

由于此过程依赖于应用程序在发出请求之前解析域名的行为，因此在 macOS 等 DNS 由系统代理和缓存的环境中可能会出现问题。
