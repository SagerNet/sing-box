---
icon: material/delete-clock
---

!!! failure "Deprecated in sing-box 1.11.0"

    Legacy special outbounds are deprecated and will be removed in sing-box 1.13.0, check [Migration](/migration/#migrate-legacy-special-outbounds-to-rule-actions).

`dns` outbound is a internal DNS server.

### Structure

```json
{
  "type": "dns",
  "tag": "dns-out"
}
```

!!! note ""

    There are no outbound connections by the DNS outbound, all requests are handled internally.

### Fields

No fields.