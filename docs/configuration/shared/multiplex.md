### Server Requirements

`sing-box` :)

### Structure

```json
{
  "enabled": true,
  "max_connections": 4,
  "min_streams": 4,
  "max_streams": 0
}
```

### Fields

#### enabled

Enable multiplex.

#### max_connections

Maximum connections.

Conflict with `max_streams`.

#### min_streams

Minimum multiplexed streams in a connection before opening a new connection.

Conflict with `min_streams`.

#### max_streams

Maximum multiplexed streams in a connection before opening a new connection.

Conflict with `max_connections` and `min_streams`.
