---
icon: material/new-box
---

!!! question "Since sing-box 1.14.0"

### Structure

```json
{
  "initial_packet_size": 0,
  "disable_path_mtu_discovery": false,

  ... // HTTP2 Fields
}
```

### Fields

#### initial_packet_size

Initial QUIC packet size.

#### disable_path_mtu_discovery

Disable QUIC path MTU discovery.

### HTTP2 Fields

See [HTTP2 Fields](/configuration/shared/http2/) for details.
