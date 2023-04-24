### Server Requirements

`sing-box` :)

### Structure

```json
{
  "enabled": true,
  "protocol": "smux",
  "max_connections": 4,
  "min_streams": 4,
  "max_streams": 0,
  "padding": false
}
```

### Fields

#### enabled

Enable multiplex.

#### protocol

Multiplex protocol.

| Protocol | Description                        |
|----------|------------------------------------|
| smux     | https://github.com/xtaci/smux      |
| yamux    | https://github.com/hashicorp/yamux |
| h2mux    | https://golang.org/x/net/http2     |

h2mux is used by default.

#### max_connections

Maximum connections.

Conflict with `max_streams`.

#### min_streams

Minimum multiplexed streams in a connection before opening a new connection.

Conflict with `max_streams`.

#### max_streams

Maximum multiplexed streams in a connection before opening a new connection.

Conflict with `max_connections` and `min_streams`.

#### padding

!!! info

    Requires sing-box server version 1.3-beta9 or later.

Enable padding.

