!!! question "Since sing-box 1.11.0"

# Endpoint

An endpoint is a protocol with inbound and outbound behavior.

### Structure

```json
{
  "endpoints": [
    {
      "type": "",
      "tag": ""
    }
  ]
}
```

### Fields

| Type        | Format                    |
|-------------|---------------------------|
| `wireguard` | [WireGuard](./wireguard/) |
| `tailscale` | [Tailscale](./tailscale/) |

#### tag

The tag of the endpoint.
