---
icon: material/new-box
---

!!! quote "sing-box 1.9.0 中的更改"

    :material-plus: [client_subnet](#client_subnet)

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

如果设置了 `server.strategy`，则不生效。

#### disable_cache

禁用 DNS 缓存。

#### disable_expire

禁用 DNS 缓存过期。

#### independent_cache

使每个 DNS 服务器的缓存独立，以满足特殊目的。如果启用，将轻微降低性能。

#### reverse_mapping

在响应 DNS 查询后存储 IP 地址的反向映射以为路由目的提供域名。

由于此过程依赖于应用程序在发出请求之前解析域名的行为，因此在 macOS 等 DNS 由系统代理和缓存的环境中可能会出现问题。

#### client_subnet

!!! question "自 sing-box 1.9.0 起"

默认情况下，将带有指定 IP 地址的 `edns0-subnet` OPT 附加记录附加到每个查询。
 
可以被 `servers.[].client_subnet` 或 `rules.[].client_subnet` 覆盖。

#### fakeip

[FakeIP](./fakeip/) 设置。
