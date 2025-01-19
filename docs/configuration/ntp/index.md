# NTP

Built-in NTP client service.

If enabled, it will provide time for protocols like TLS/Shadowsocks/VMess, which is useful for environments where time
synchronization is not possible.

### Structure

```json
{
  "ntp": {
    "enabled": false,
    "server": "time.apple.com",
    "server_port": 123,
    "interval": "30m",
    "write_to_system": false

    ... // Dial Fields
  }
}

```

### Fields

#### enabled

Enable NTP service.

#### server

==Required==

NTP server address.

#### server_port

NTP server port.

123 is used by default.

#### interval

Time synchronization interval.

30 minutes is used by default.

#### write_to_system

Write time to system.

false is used by default.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.