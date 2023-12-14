`socks` outbound is a socks4/socks4a/socks5 client.

### Structure

```json
{
  "type": "socks",
  "tag": "socks-out",
  
  "server": "127.0.0.1",
  "server_port": 1080,
  "version": "5",
  "username": "sekai",
  "password": "admin",
  "network": "udp",
  "udp_over_tcp": false | {},

  ... // Dial Fields
}
```

### Fields

#### server

==Required==

The server address.

#### server_port

==Required==

The server port.

#### version

The SOCKS version, one of `4` `4a` `5`.

SOCKS5 used by default.

#### username

SOCKS username.

#### password

SOCKS5 password.

#### network

Enabled network

One of `tcp` `udp`.

Both is enabled by default.

#### udp_over_tcp

UDP over TCP protocol settings.

See [UDP Over TCP](/configuration/shared/udp-over-tcp/) for details.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
