---
icon: material/alert-decagram
---

!!! quote "Changes in sing-box 1.12.0"

    :material-decagram: [servers](#servers)

!!! quote "Changes in sing-box 1.11.0"

    :material-plus: [cache_capacity](#cache_capacity)

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
    "independent_cache": false,
    "cache_capacity": 0,
    "reverse_mapping": false,
    "client_subnet": "",
    "fakeip": {}
  }
}

```

### Fields

| Key      | Format                          |
|----------|---------------------------------|
| `server` | List of [DNS Server](./server/) |
| `rules`  | List of [DNS Rule](./rule/)     |
| `fakeip` | [FakeIP](./fakeip/)             |

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

#### independent_cache

Make each DNS server's cache independent for special purposes. If enabled, will slightly degrade performance.

#### cache_capacity

!!! question "Since sing-box 1.11.0"

LRU cache capacity.

Value less than 1024 will be ignored.

#### reverse_mapping

Stores a reverse mapping of IP addresses after responding to a DNS query in order to provide domain names when routing.

Since this process relies on the act of resolving domain names by an application before making a request, it can be
problematic in environments such as macOS, where DNS is proxied and cached by the system.

#### client_subnet

!!! question "Since sing-box 1.9.0"

Append a `edns0-subnet` OPT extra record with the specified IP prefix to every query by default.

If value is an IP address instead of prefix, `/32` or `/128` will be appended automatically.

Can be overrides by `servers.[].client_subnet` or `rules.[].client_subnet`.
