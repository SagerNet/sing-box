---
icon: material/new-box
---

!!! quote "sing-box 1.14.0 中的更改"

    :material-delete-clock: [strategy](#strategy)  
    :material-plus: [evaluate](#evaluate)  
    :material-plus: [respond](#respond)  
    :material-plus: [disable_optimistic_cache](#disable_optimistic_cache)  
    :material-plus: [timeout](#timeout)  
    :material-plus: [race](#race)  
    :material-plus: [speculative](#speculative)

!!! quote "sing-box 1.12.0 中的更改"

    :material-plus: [strategy](#strategy)  
    :material-plus: [predefined](#predefined)

!!! question "自 sing-box 1.11.0 起"

### 结构

```json
{
  "action": "",
  "race": false,

  ... // 动作字段
}
```

#### action

要执行的动作。默认使用 `route`。

#### race

!!! question "自 sing-box 1.14.0 起"

仅可用于 `route`、`respond`、`reject` 和 `predefined` 动作。

需要 [`match_response`](/zh/configuration/dns/rule/#match_response)（对 logical 规则，位于子规则中）。
与 `speculative` 冲突。

默认情况下，规则逐条按顺序匹配：带 `match_response` 的规则等待其引用的响应，在它被判定之前不会匹配任何后续规则。

启用 `race` 的规则成为竞态规则，不再保持这一顺序：其引用的响应尚未到达时，规则匹配会越过它继续进行，因此竞态规则的匹配相互并行、也与后续规则并行。每条竞态规则在其引用的响应可用时被判定，首个匹配的竞态规则立即终止规则评估，其余查询将被取消。

未启用 `race` 的规则仍严格按顺序生效：只要前面还有未判定的竞态规则，其他已匹配规则的动作就被扣住，直到所有竞态规则均未匹配。因此只有竞态规则之间的结果取决于服务器速度。

### route

```json
{
  "action": "route", // 默认
  "server": "",
  "speculative": false,
  "strategy": "",
  "disable_cache": false,
  "disable_optimistic_cache": false,
  "rewrite_ttl": null,
  "timeout": "",
  "client_subnet": null
}
```

`route` 继承了将 DNS 请求 路由到指定服务器的经典规则动作。

#### server

==必填==

目标 DNS 服务器的标签。

#### speculative

!!! question "自 sing-box 1.14.0 起"

与 `race` 冲突。没有前序竞态规则时无效果。

默认情况下，查询决不与未判定的竞态规则并行发出：已匹配的 `route` 动作扣住其查询，直到所有竞态规则均未匹配后才发送。

启用 `speculative` 后，查询成为投机查询：在规则匹配时立即发出、与未判定的竞态规则并行，且可能被浪费；其响应仍仅在所有竞态规则均未匹配后才被使用。

#### strategy

!!! question "自 sing-box 1.12.0 起"

!!! failure "已在 sing-box 1.14.0 废弃"

    `strategy` 已在 sing-box 1.14.0 废弃，且将在 sing-box 1.16.0 中被移除。

为此查询设置域名策略。

可选项：`prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`。

#### disable_cache

在此查询中禁用缓存。

#### disable_optimistic_cache

!!! question "自 sing-box 1.14.0 起"

在此查询中禁用乐观 DNS 缓存。

#### rewrite_ttl

重写 DNS 回应中的 TTL。

#### timeout

!!! question "自 sing-box 1.14.0 起"

覆盖匹配查询的 DNS 查询超时时间。

将覆盖 `dns.timeout`。

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
  "tag": "",
  "speculative": false,
  "disable_cache": false,
  "disable_optimistic_cache": false,
  "rewrite_ttl": null,
  "timeout": "",
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

#### tag

已评估响应的标签。

带标签的响应仅能通过 [`match_response`](/zh/configuration/dns/rule/#match_response) 以标签引用；
`match_response: true` 引用最近一条无 `tag` 的 `evaluate` 动作的响应。

#### speculative

!!! question "自 sing-box 1.14.0 起"

没有前序竞态规则时无效果。

默认情况下，查询决不与未判定的竞态规则并行发出：已匹配的 `evaluate` 动作扣住其查询，规则匹配在此处停止，直到所有竞态规则均未匹配。

启用 `speculative` 后，查询成为投机查询：在规则匹配时立即发出、与未判定的竞态规则并行，且可能被浪费；规则匹配继续进行而不等待竞态规则。

#### disable_cache

在此查询中禁用缓存。

#### disable_optimistic_cache

!!! question "自 sing-box 1.14.0 起"

在此查询中禁用乐观 DNS 缓存。

#### rewrite_ttl

重写 DNS 回应中的 TTL。

#### timeout

!!! question "自 sing-box 1.14.0 起"

覆盖匹配查询的 DNS 查询超时时间。

将覆盖 `dns.timeout`。

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

此动作不会发起新的 DNS 查询。

只能用于前面已有顶层 `evaluate` 规则的场景。如果运行时命中该动作时没有已评估的响应，则请求会直接返回错误，而不是继续匹配后续规则。

### route-options

```json
{
  "action": "route-options",
  "disable_cache": false,
  "disable_optimistic_cache": false,
  "rewrite_ttl": null,
  "timeout": "",
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
