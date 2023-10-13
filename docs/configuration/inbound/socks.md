`socks` inbound is a socks4, socks4a, socks5 server.

### Structure

```json
{
  "type": "socks",
  "tag": "socks-in",

  ... // Listen Fields

  "users": [
    {
      "username": "admin",
      "password": "admin"
    }
  ]
}
```

### Listen Fields

See [Listen Fields](/configuration/shared/listen) for details.

!!! error ""

    The support for UDP follows [RFC 1928](https://datatracker.ietf.org/doc/html/rfc1928), will use random available UDP ports, as opposed to other popular proxy programs which use [fixed UDP port](https://github.com/v2fly/v2fly-github-io/issues/104).

### Fields

#### users

SOCKS users.

No authentication required if empty.
