### Server Requirements

`sing-box` :)

### Structure

```json
{
  "enabled": true,
  "protocol": "smux",
  "max_connections": 4,
  "min_streams": 4,
  "max_streams": 0
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

SMux is used by default.

#### max_connections

Maximum connections.

Conflict with `max_streams`.

#### min_streams

Minimum multiplexed streams in a connection before opening a new connection.

Conflict with `min_streams`.

#### max_streams

Maximum multiplexed streams in a connection before opening a new connection.

Conflict with `max_connections` and `min_streams`.
