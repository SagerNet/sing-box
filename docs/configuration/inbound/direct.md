`direct` inbound is a tunnel server.

### Structure

```json
{
  "type": "direct",
  "tag": "direct-in",
  
  ... // Listen Fields

  "network": "udp",
  "override_address": "1.0.0.1",
  "override_port": 53
}
```

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.

### Fields

#### network

Listen network, one of `tcp` `udp`.

Both if empty.

#### override_address

Override the connection destination address.

#### override_port

Override the connection destination port.