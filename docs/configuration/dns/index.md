### Structure

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

### Fields

| Key      | Format                         |
|----------|--------------------------------|
| `server` | List of [DNS Server](./server) |
| `rules`  | List of [DNS Rule](./rule)     |

#### final

Default dns server tag.

The first server will be used if empty.

#### strategy

Default domain strategy for resolving the domain names.

One of `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`.

#### disable_cache

Disable dns cache.

#### disable_expire

Disable dns cache expire.