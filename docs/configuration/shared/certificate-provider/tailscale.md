---
icon: material/new-box
---

!!! question "Since sing-box 1.14.0"

# Tailscale

### Structure

```json
{
  "type": "tailscale",
  "tag": "ts-cert",
  "endpoint": "ts-ep"
}
```

### Fields

#### endpoint

==Required==

The tag of the [Tailscale endpoint](/configuration/endpoint/tailscale/) to reuse.

[MagicDNS and HTTPS](https://tailscale.com/kb/1153/enabling-https) must be enabled in the Tailscale admin console.
