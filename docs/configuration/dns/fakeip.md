---
icon: material/note-remove
---

!!! failure "Removed in sing-box 1.14.0"

    Legacy fake-ip configuration is deprecated in sing-box 1.12.0 and removed in sing-box 1.14.0, check [Migration](/migration/#migrate-to-new-dns-server-formats).

### Structure

```json
{
  "enabled": true,
  "inet4_range": "198.18.0.0/15",
  "inet6_range": "fc00::/18"
}
```

### Fields

#### enabled

Enable FakeIP service.

#### inet4_range

IPv4 address range for FakeIP.

#### inet6_range

IPv6 address range for FakeIP.
