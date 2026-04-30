---
icon: material/new-box
---

!!! question "Since sing-box 1.14.0"

# mDNS

### Structure

```json
{
  "dns": {
    "servers": [
      {
        "type": "mdns",
        "tag": "",

        "interface": [],

        // Dial Fields
      }
    ]
  }
}
```

!!! info ""

    You usually do not need an explicit `mdns` server in addition to a [Local](./local/) server: the local server already routes queries for `*.local.` and IPv4/IPv6 link-local reverse zones via mDNS on non-Apple platforms and via the system resolver on Apple platforms. Add an explicit `mdns` server only when you want to reference it from [`preferred_by`](../rule/#preferred_by) or use it standalone.

### Fields

#### interface

List of network interface names to send mDNS queries on.

When empty, all interfaces that are up, multicast-capable, and non-loopback are used.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
