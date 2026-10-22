---
icon: material/new-box
---

!!! question "Since sing-box 1.12.0"

# Fake IP

### Structure

```json
{
  "dns": {
    "servers": [
      {
        "type": "fakeip",
        "tag": "",

        "inet4_range": "198.18.0.0/15",
        "inet6_range": "2001:2::/48"
      }
    ]
  }
}
```

### Fields

#### inet4_range

IPv4 address range for FakeIP.

#### inet6_range

IPv6 address range for FakeIP.
