---
icon: material/delete-clock
---

!!! failure "Deprecated in sing-box 1.12.0"

    Legacy fake-ip configuration is deprecated and will be removed in sing-box 1.14.0, check [Migration](/migration/#migrate-to-new-dns-servers).

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

#### inet6_address

IPv6 address range for FakeIP.
