---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.14.0"

    :material-delete-clock: [strategy](#strategy)  
    :material-plus: [evaluate](#evaluate)  
    :material-plus: [respond](#respond)  
    :material-plus: [disable_optimistic_cache](#disable_optimistic_cache)  
    :material-plus: [timeout](#timeout)  
    :material-plus: [race](#race)  
    :material-plus: [speculative](#speculative)

!!! quote "Changes in sing-box 1.12.0"

    :material-plus: [strategy](#strategy)  
    :material-plus: [predefined](#predefined)

!!! question "Since sing-box 1.11.0"

### Structure

```json
{
  "action": "",
  "race": false,

  ... // Action Fields
}
```

#### action

The action to perform. `route` will be used by default.

#### race

!!! question "Since sing-box 1.14.0"

Only available with `route`, `respond`, `reject` and `predefined` actions.

Requires [`match_response`](/configuration/dns/rule/#match_response) (for logical rules, in sub-rules).
Conflict with `speculative`.

By default, rules are matched one after another in listed order: a rule with `match_response`
waits for its referenced responses, and no later rule is matched until it has been judged.

A rule with `race` enabled does not hold this order: rule matching continues past it while its
referenced responses are still pending, so the matching of race rules runs in parallel — with
each other and with the rules after them. Each race rule is judged once its referenced
responses are available, and the first race rule that matches terminates rule evaluation
immediately; the remaining queries are canceled.

Rules without `race` still take effect strictly in listed order: while a preceding race rule is
not yet judged, the action of any other matched rule is held until none of the race rules
matched. The result may therefore depend on server speed only among race rules.

### route

```json
{
  "action": "route",  // default
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

`route` inherits the classic rule behavior of routing DNS requests to the specified server.

#### server

==Required==

Tag of target server.

#### speculative

!!! question "Since sing-box 1.14.0"

Conflict with `race`. Has no effect without a preceding `race` rule.

By default, no query is sent in parallel with pending race rules: a matched `route` action
holds its query until none of the race rules matched.

When `speculative` is enabled, the query is sent as soon as the rule matches, in parallel with
the pending race rules, and may be wasted: its response is still used only after none of the
race rules matched.

#### strategy

!!! question "Since sing-box 1.12.0"

!!! failure "Deprecated in sing-box 1.14.0"

    `strategy` is deprecated in sing-box 1.14.0 and will be removed in sing-box 1.16.0.

Set domain strategy for this query.

One of `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`.

#### disable_cache

Disable cache and save cache in this query.

#### disable_optimistic_cache

!!! question "Since sing-box 1.14.0"

Disable optimistic DNS caching in this query.

#### rewrite_ttl

Rewrite TTL in DNS responses.

#### timeout

!!! question "Since sing-box 1.14.0"

Override the DNS query timeout for matched queries.

Will override `dns.timeout`.

#### client_subnet

Append a `edns0-subnet` OPT extra record with the specified IP prefix to every query by default.

If value is an IP address instead of prefix, `/32` or `/128` will be appended automatically.

Will override `dns.client_subnet`.

### evaluate

!!! question "Since sing-box 1.14.0"

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

`evaluate` sends a DNS query to the specified server and saves the evaluated response for subsequent rules
to match against using [`match_response`](/configuration/dns/rule/#match_response) and response fields.
Unlike `route`, it does **not** terminate rule evaluation.

Only allowed on top-level DNS rules (not inside logical sub-rules).
Rules that use [`match_response`](/configuration/dns/rule/#match_response) or Response Match Fields
require a preceding top-level rule with `evaluate` action. A rule's own `evaluate` action
does not satisfy this requirement, because matching happens before the action runs.

#### server

==Required==

Tag of target server.

#### tag

Tag of the evaluated response.

A tagged response is only referenced via [`match_response`](/configuration/dns/rule/#match_response) with the tag;
`match_response: true` references the response of the latest `evaluate` action without `tag`.

#### speculative

!!! question "Since sing-box 1.14.0"

Has no effect without a preceding `race` rule.

By default, no query is sent in parallel with pending race rules: a matched `evaluate` action
holds its query, and rule matching stops there, until none of the race rules matched.

When `speculative` is enabled, the query is sent as soon as the rule matches, in parallel with
the pending race rules, and may be wasted: rule matching continues without waiting for them.

#### disable_cache

Disable cache and save cache in this query.

#### disable_optimistic_cache

!!! question "Since sing-box 1.14.0"

Disable optimistic DNS caching in this query.

#### rewrite_ttl

Rewrite TTL in DNS responses.

#### timeout

!!! question "Since sing-box 1.14.0"

Override the DNS query timeout for matched queries.

Will override `dns.timeout`.

#### client_subnet

Append a `edns0-subnet` OPT extra record with the specified IP prefix to every query by default.

If value is an IP address instead of prefix, `/32` or `/128` will be appended automatically.

Will override `dns.client_subnet`.

### respond

!!! question "Since sing-box 1.14.0"

```json
{
  "action": "respond"
}
```

`respond` terminates rule evaluation and returns the evaluated response from a preceding [`evaluate`](/configuration/dns/rule_action/#evaluate) action.

This action does not send a new DNS query.

Only allowed after a preceding top-level `evaluate` rule. If the action is reached without an evaluated response at runtime, the request fails with an error instead of falling through to later rules.

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

`route-options` set options for routing.

### reject

```json
{
  "action": "reject",
  "method": "",
  "no_drop": false
}
```

`reject` reject DNS requests.

#### method

- `default`: Reply with REFUSED.
- `drop`: Drop the request.

`default` will be used by default.

#### no_drop

If not enabled, `method` will be temporarily overwritten to `drop` after 50 triggers in 30s.

Not available when `method` is set to drop.

### predefined

!!! question "Since sing-box 1.12.0"

```json
{
  "action": "predefined",
  "rcode": "",
  "answer": [],
  "ns": [],
  "extra": []
}
```

`predefined` responds with predefined DNS records.

#### rcode

The response code.

| Value      | Value in the legacy rcode server | Description     |
|------------|----------------------------------|-----------------|
| `NOERROR`  | `success`                        | Ok              |
| `FORMERR`  | `format_error`                   | Bad request     |
| `SERVFAIL` | `server_failure`                 | Server failure  |
| `NXDOMAIN` | `name_error`                     | Not found       |
| `NOTIMP`   | `not_implemented`                | Not implemented |
| `REFUSED`  | `refused`                        | Refused         |

`NOERROR` will be used by default.

#### answer

List of text DNS record to respond as answers.

Examples:

| Record Type | Example                       |
|-------------|-------------------------------|
| `A`         | `localhost. IN A 127.0.0.1`   |
| `AAAA`      | `localhost. IN AAAA ::1`      |
| `TXT`       | `localhost. IN TXT \"Hello\"` |

#### ns

List of text DNS record to respond as name servers.

#### extra

List of text DNS record to respond as extra records.
