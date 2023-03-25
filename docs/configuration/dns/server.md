### Structure

```json
{
  "dns": {
    "servers": [
      {
        "tag": "google",
        "address": "tls://dns.google",
        "address_resolver": "local",
        "address_strategy": "prefer_ipv4",
        "strategy": "ipv4_only",
        "detour": "direct"
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

| Protocol            | Format                        |
|---------------------|-------------------------------|
| `System`            | `local`                       |
| `TCP`               | `tcp://1.0.0.1`               |
| `UDP`               | `8.8.8.8` `udp://8.8.4.4`     |
| `TLS`               | `tls://dns.google`            |
| `HTTPS`             | `https://1.1.1.1/dns-query`   |
| `QUIC`              | `quic://dns.adguard.com`      |
| `HTTP3`             | `h3://8.8.8.8/dns-query`      |
| `RCode`             | `rcode://refused`             |
| `DHCP`              | `dhcp://auto` or `dhcp://en0` |
|  [FakeIP](./fakeip) | `fakeip`                      |

!!! warning ""

    To ensure that system DNS is in effect, rather than Go's built-in default resolver, enable CGO at compile time.

!!! warning ""

    QUIC and HTTP3 transport is not included by default, see [Installation](/#installation).

!!! info ""

    the RCode transport is often used to block queries. Use with rules and the `disable_cache` rule option.

!!! warning ""

    DHCP transport is not included by default, see [Installation](/#installation).

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

Take no effect if override by other settings.

#### detour

Tag of an outbound for connecting to the dns server.

Default outbound will be used if empty.