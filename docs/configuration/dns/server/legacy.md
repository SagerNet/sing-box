---
icon: material/delete-clock
---

!!! failure "Deprecated in sing-box 1.12.0"

    Legacy DNS servers is deprecated and will be removed in sing-box 1.14.0, check [Migration](/migration/#migrate-to-new-dns-servers).

!!! quote "Changes in sing-box 1.9.0"

    :material-plus: [client_subnet](#client_subnet)

### Structure

```json
{
  "dns": {
    "servers": [
      {
        "tag": "",
        "address": "",
        "address_resolver": "",
        "address_strategy": "",
        "strategy": "",
        "detour": "",
        "client_subnet": ""
      }
    ]
  }
}
```

### Fields

#### tag

The tag of the dns server.

#### address

==Required==

The address of the dns server.

| Protocol                             | Format                        |
|--------------------------------------|-------------------------------|
| `System`                             | `local`                       |
| `TCP`                                | `tcp://1.0.0.1`               |
| `UDP`                                | `8.8.8.8` `udp://8.8.4.4`     |
| `TLS`                                | `tls://dns.google`            |
| `HTTPS`                              | `https://1.1.1.1/dns-query`   |
| `QUIC`                               | `quic://dns.adguard.com`      |
| `HTTP3`                              | `h3://8.8.8.8/dns-query`      |
| `RCode`                              | `rcode://refused`             |
| `DHCP`                               | `dhcp://auto` or `dhcp://en0` |
| [FakeIP](/configuration/dns/fakeip/) | `fakeip`                      |

!!! warning ""

    To ensure that Android system DNS is in effect, rather than Go's built-in default resolver, enable CGO at compile time.

!!! info ""

    the RCode transport is often used to block queries. Use with rules and the `disable_cache` rule option.

| RCode             | Description           | 
|-------------------|-----------------------|
| `success`         | `No error`            |
| `format_error`    | `Format error`        |
| `server_failure`  | `Server failure`      |
| `name_error`      | `Non-existent domain` |
| `not_implemented` | `Not implemented`     |
| `refused`         | `Query refused`       |

#### address_resolver

==Required if address contains domain==

Tag of a another server to resolve the domain name in the address.

#### address_strategy

The domain strategy for resolving the domain name in the address.

One of `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`.

`dns.strategy` will be used if empty.

#### strategy

Default domain strategy for resolving the domain names.

One of `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`.

Take no effect if overridden by other settings.

#### detour

Tag of an outbound for connecting to the dns server.

Default outbound will be used if empty.

#### client_subnet

!!! question "Since sing-box 1.9.0"

Append a `edns0-subnet` OPT extra record with the specified IP prefix to every query by default.

If value is an IP address instead of prefix, `/32` or `/128` will be appended automatically.

Can be overrides by `rules.[].client_subnet`.

Will overrides `dns.client_subnet`.
