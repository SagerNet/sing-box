---
icon: material/alert-decagram
---

!!! quote "sing-box 1.14.0 中的更改"

    :material-delete-clock: [independent_cache](#independent_cache)  
    :material-plus: [optimistic](#optimistic)  
    :material-plus: [timeout](#timeout)

!!! quote "sing-box 1.12.0 中的更改"

    :material-decagram: [servers](#servers)

!!! quote "sing-box 1.11.0 中的更改"

    :material-plus: [cache_capacity](#cache_capacity)

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
    "independent_cache": false,
    "cache_capacity": 0,
    "optimistic": false, // or {}
    "timeout": "",
    "reverse_mapping": false,
    "client_subnet": "",
    "fakeip": {}
  }
}

```

### 字段

| 键        | 格式                      |
|----------|-------------------------|
| `server` | 一组 [DNS 服务器](./server/) |
| `rules`  | 一组 [DNS 规则](./rule/)    |

#### final

默认 DNS 服务器的标签。

默认使用第一个服务器。

#### strategy

默认解析域名策略。

可选值: `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`。

#### disable_cache

禁用 DNS 缓存。

与 `optimistic` 冲突。

#### disable_expire

禁用 DNS 缓存过期。

与 `optimistic` 冲突。

#### independent_cache

!!! failure "已在 sing-box 1.14.0 废弃"

    `independent_cache` 已在 sing-box 1.14.0 废弃，且将在 sing-box 1.16.0 中被移除，参阅[迁移指南](/zh/migration/#迁移-independent-dns-cache)。

使每个 DNS 服务器的缓存独立，以满足特殊目的。如果启用，将轻微降低性能。

#### cache_capacity

!!! question "自 sing-box 1.11.0 起"

LRU 缓存容量。

小于 1024 的值将被忽略。

#### optimistic

!!! question "自 sing-box 1.14.0 起"

启用乐观 DNS 缓存。当缓存的 DNS 条目已过期但仍在超时窗口内时，
立即返回过期的响应，同时在后台触发刷新。

与 `disable_cache` 和 `disable_expire` 冲突。

接受布尔值或对象。当设置为 `true` 时，使用默认超时 `3d`。

```json
{
  "enabled": true,
  "timeout": "3d"
}
```

##### enabled

启用乐观 DNS 缓存。

##### timeout

过期缓存条目可被乐观提供的最长时间。

默认使用 `3d`。

#### timeout

!!! question "自 sing-box 1.14.0 起"

每次 DNS 查询的默认超时时间。

默认使用 `10s`。

可被 `rules.[].timeout`（DNS 规则动作）或 `domain_resolver.timeout` 覆盖。

#### reverse_mapping

在响应 DNS 查询后存储 IP 地址的反向映射以为路由目的提供域名。

由于此过程依赖于应用程序在发出请求之前解析域名的行为，因此在 macOS 等 DNS 由系统代理和缓存的环境中可能会出现问题。

#### client_subnet

!!! question "自 sing-box 1.9.0 起"

默认情况下，将带有指定 IP 前缀的 `edns0-subnet` OPT 附加记录附加到每个查询。

如果值是 IP 地址而不是前缀，则会自动附加 `/32` 或 `/128`。

可以被 `servers.[].client_subnet` 或 `rules.[].client_subnet` 覆盖。

#### fakeip :material-note-remove:

[FakeIP](./fakeip/) 设置。
