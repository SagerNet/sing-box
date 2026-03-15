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

The tag of the [Tailscale endpoint](/configuration/endpoint/tailscale/) to reuse.

The certificate provider uses the endpoint's embedded `tsnet` node and obtains
certificates through Tailscale's local API, similar to `tsnet.ListenTLS`.

The referenced endpoint must already be configured for the desired tailnet, and
MagicDNS and HTTPS must be enabled in the Tailscale admin panel for certificate
issuance to succeed.
