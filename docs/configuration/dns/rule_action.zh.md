---
icon: material/new-box
---

!!! quote "sing-box 1.14.0 中的更改"

    :material-delete-clock: [strategy](#strategy)  
    :material-plus: [evaluate](#evaluate)  
    :material-plus: [respond](#respond)

!!! quote "sing-box 1.12.0 中的更改"

    :material-plus: [strategy](#strategy)  
    :material-plus: [predefined](#predefined)

!!! question "自 sing-box 1.11.0 起"

### route

```json
{
  "action": "route", // 默认
  "server": "",
  "strategy": "",
  "disable_cache": false,
  "rewrite_ttl": null,
  "client_subnet": null
}
```

`route` 继承了将 DNS 请求 路由到指定服务器的经典规则动作。

#### server

==必填==

目标 DNS 服务器的标签。

#### strategy

!!! question "自 sing-box 1.12.0 起"

!!! failure "已在 sing-box 1.14.0 废弃"

    `strategy` 已在 sing-box 1.14.0 废弃，且将在 sing-box 1.16.0 中被移除。

为此查询设置域名策略。已废弃，参阅[迁移指南](/zh/migration/#迁移-dns-规则动作-strategy-到规则项)。

可选项：`prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`。

#### disable_cache

在此查询中禁用缓存。

#### rewrite_ttl

重写 DNS 回应中的 TTL。

#### client_subnet

默认情况下，将带有指定 IP 前缀的 `edns0-subnet` OPT 附加记录附加到每个查询。

如果值是 IP 地址而不是前缀，则会自动附加 `/32` 或 `/128`。

将覆盖 `dns.client_subnet`.

### evaluate

!!! question "自 sing-box 1.14.0 起"

```json
{
  "action": "evaluate",
  "server": "",
  "disable_cache": false,
  "rewrite_ttl": null,
  "client_subnet": null
}
```

`evaluate` 向指定服务器发送 DNS 查询并保存已评估的响应，供后续规则通过 [`match_response`](/zh/configuration/dns/rule/#match_response) 和响应字段进行匹配。与 `route` 不同，它**不会**终止规则评估。

仅允许在顶层 DNS 规则中使用（不可在逻辑子规则内部使用）。
使用 [`match_response`](/zh/configuration/dns/rule/#match_response) 或响应匹配字段的规则，
需要位于更早的顶层 `evaluate` 规则之后。规则自身的 `evaluate` 动作不能满足这个条件，
因为匹配发生在动作执行之前。

#### server

==必填==

目标 DNS 服务器的标签。

#### disable_cache

在此查询中禁用缓存。

#### rewrite_ttl

重写 DNS 回应中的 TTL。

#### client_subnet

默认情况下，将带有指定 IP 前缀的 `edns0-subnet` OPT 附加记录附加到每个查询。

如果值是 IP 地址而不是前缀，则会自动附加 `/32` 或 `/128`。

将覆盖 `dns.client_subnet`.

### respond

!!! question "自 sing-box 1.14.0 起"

```json
{
  "action": "respond"
}
```

`respond` 会终止规则评估，并直接返回前序 [`evaluate`](/zh/configuration/dns/rule_action/#evaluate) 动作保存的已评估的响应。

此动作不会发起新的 DNS 查询，也没有额外选项。

只能用于前面已有顶层 `evaluate` 规则的场景。如果运行时命中该动作时没有已评估的响应，则请求会直接返回错误，而不是继续匹配后续规则。

### route-options

```json
{
  "action": "route-options",
  "disable_cache": false,
  "rewrite_ttl": null,
  "client_subnet": null
}
```

`route-options` 为路由设置选项。

### reject

```json
{
  "action": "reject",
  "method": "",
  "no_drop": false
}
```

`reject` 拒绝 DNS 请求。

#### method

- `default`: 返回 REFUSED。
- `drop`: 丢弃请求。

默认使用 `default`。

#### no_drop

如果未启用，则 30 秒内触发 50 次后，`method` 将被暂时覆盖为 `drop`。

当 `method` 设为 `drop` 时不可用。

### predefined

!!! question "自 sing-box 1.12.0 起"

```json
{
  "action": "predefined",
  "rcode": "",
  "answer": [],
  "ns": [],
  "extra": []
}
```

`predefined` 以预定义的 DNS 记录响应。

#### rcode

响应码。

| 值          | 旧 rcode DNS 服务器中的值 | 描述              |
|------------|--------------------|-----------------|
| `NOERROR`  | `success`          | Ok              |
| `FORMERR`  | `format_error`     | Bad request     |
| `SERVFAIL` | `server_failure`   | Server failure  |
| `NXDOMAIN` | `name_error`       | Not found       |
| `NOTIMP`   | `not_implemented`  | Not implemented |
| `REFUSED`  | `refused`          | Refused         |

默认使用 `NOERROR`。

#### answer

用于作为回答响应的文本 DNS 记录列表。

例子:

| 记录类型   | 例子                            |
|--------|-------------------------------|
| `A`    | `localhost. IN A 127.0.0.1`   |
| `AAAA` | `localhost. IN AAAA ::1`      |
| `TXT`  | `localhost. IN TXT \"Hello\"` |

#### ns

用于作为名称服务器响应的文本 DNS 记录列表。

#### extra

用于作为额外记录响应的文本 DNS 记录列表。
