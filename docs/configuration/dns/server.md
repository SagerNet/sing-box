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

The address of the dns server.

| Protocol | Format                      |
|----------|-----------------------------|
| `System` | `local`                     |
| `TCP`    | `tcp://1.0.0.1`             |
| `UDP`    | `8.8.8.8` `udp://8.8.4.4`   |
| `TLS`    | `tls://dns.google`          |
| `HTTPS`  | `https://1.1.1.1/dns-query` |

!!! warning ""

    To ensure that system DNS is in effect, rather than go's built-in default resolver, enable CGO at compile time.

#### address_resolver

Tag of a another server to resolve the domain name in the address.

#### address_strategy

The domain strategy for resolving the domain name in the address.

One of `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`.

`dns.strategy` will be used if empty.

#### detour

Tag of an outbound for connecting to the dns server.

Requests will be sent directly if the empty.