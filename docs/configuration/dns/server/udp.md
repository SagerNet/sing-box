---
icon: material/new-box
---

!!! question "Since sing-box 1.12.0"

# UDP

### Structure

```json
{
  "dns": {
    "servers": [
      {
        "type": "udp",
        "tag": "",
        
        "server": "",
        "server_port": 53,
        
        // Dial Fields
      }
    ]
  }
}
```

!!! info "Difference from legacy UDP server"

    * The old server uses default outbound by default unless detour is specified; the new one uses dialer just like outbound, which is equivalent to using an empty direct outbound by default.
    * The old server uses `address_resolver` and `address_strategy` to resolve the domain name in the server; the new one uses `domain_resolver` and `domain_strategy` in [Dial Fields](/configuration/shared/dial/) instead.

### Fields

#### server

==Required==

The address of the DNS server.

If domain name is used, `domain_resolver` must also be set to resolve IP address.

#### server_port

The port of the DNS server.

`53` will be used by default.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
