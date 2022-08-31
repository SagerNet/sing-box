`direct` outbound send requests directly.

### Structure

```json
{
  "type": "direct",
  "tag": "direct-out",
  
  "override_address": "1.0.0.1",
  "override_port": 53,
  "proxy_protocol": 0,
  
  ... // Dial Fields
}
```

### Fields

#### override_address

Override the connection destination address.

#### override_port

Override the connection destination port.

#### proxy_protocol

Write [Proxy Protocol](https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt) in the connection header.

Protocol value can be `1` or `2`.

### Dial Fields

See [Dial Fields](/configuration/shared/dial) for details.
