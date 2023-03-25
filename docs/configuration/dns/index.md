# DNS

### Structure

```json
{
  "dns": {
    "servers": [],
    "rules": [],
    "final": "",
    "strategy": "",
    "disable_cache": false,
    "disable_expire": false,
    "reverse_mapping": false,
    "fakeip": {}
  }
}

```

### Fields

| Key      | Format                         |
|----------|--------------------------------|
| `server` | List of [DNS Server](./server) |
| `rules`  | List of [DNS Rule](./rule)     |
| `fakeip` | [FakeIP](./fakeip)             |

#### final

Default dns server tag.

The first server will be used if empty.

#### strategy

Default domain strategy for resolving the domain names.

One of `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`.

Take no effect if `server.strategy` is set.

#### disable_cache

Disable dns cache.

#### disable_expire

Disable dns cache expire.

#### reverse_mapping

Stores a reverse mapping of IP addresses after responding to a DNS query in order to provide domain names when routing.

Since this process relies on the act of resolving domain names by an application before making a request, it can be
problematic in environments such as macOS, where DNS is proxied and cached by the system.

#### fakeip

[FakeIP](./fakeip) settings.
