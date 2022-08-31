`mixed` inbound is a socks4, socks4a, socks5 and http server.

### Structure

```json
{
  "type": "mixed",
  "tag": "mixed-in",

  ... // Listen Fields

  "users": [
    {
      "username": "admin",
      "password": "admin"
    }
  ],
  "set_system_proxy": false
}
```

### Listen Fields

See [Listen Fields](/configuration/shared/listen) for details.

### Fields

#### users

SOCKS and HTTP users.

No authentication required if empty.

#### set_system_proxy

!!! error ""

    Only supported on Linux, Android, Windows, and macOS.

Automatically set system proxy configuration when start and clean up when stop.