---
icon: material/new-box
---

!!! question "Since sing-box 1.12.0"

# DNS over TLS (DoT)

### Structure

```json
{
  "dns": {
    "servers": {
      "type": "tls",
      "tag": "",
      
      "server": "",
      "server_port": 853,
      
      "tls": {},
      
      // Dial Fields
    }
  }
}
```

!!! info "Difference from legacy TLS server"

    * The old server uses default outbound by default unless detour is specified; the new one uses dialer just like outbound, which is equivalent to using an empty direct outbound by default.
    * The old server uses `address_resolver` and `address_strategy` to resolve the domain name in the server; the new one uses `domain_resolver` and `domain_strategy` in [Dial Fields](/configuration/shared/dial/) instead.

### Fields

#### server

==Required==

The address of the DNS server.

If domain name is used, `domain_resolver` must also be set to resolve IP address.

#### server_port

The port of the DNS server.

`853` will be used by default.

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
