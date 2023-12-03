!!! quote ""

    V2Ray API is not included by default, see [Installation](/installation/build-from-source/#build-tags).

### Structure

```json
{
  "listen": "127.0.0.1:8080",
  "stats": {
    "enabled": true,
    "inbounds": [
      "socks-in"
    ],
    "outbounds": [
      "proxy",
      "direct"
    ],
    "users": [
      "sekai"
    ]
  }
}
```

### Fields

#### listen

gRPC API listening address. V2Ray API will be disabled if empty.

#### stats

Traffic statistics service settings.

#### stats.enabled

Enable statistics service.

#### stats.inbounds

Inbound list to count traffic.

#### stats.outbounds

Outbound list to count traffic.

#### stats.users

User list to count traffic.