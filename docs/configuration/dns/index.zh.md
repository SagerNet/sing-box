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

| 关键字      | 格式                        |
|----------|---------------------------|
| `server` | 详见 [DNS Server](./server) |
| `rules`  | 详见 [DNS Rule](./rule)     |

#### final

默认 dns 服务器标签。

如果为空，将使用第一个 dns 服务器。

#### strategy

默认域名解析域策略。

可选参数有：`prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`。

设置此字段后 `server.strategy` 将无效。

#### disable_cache

禁用 dns 缓存。

#### disable_expire

禁用 dns 缓存过期。