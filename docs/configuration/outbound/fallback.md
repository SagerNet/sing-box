### Structure

```json
{
  "type": "fallback",
  "tag": "fb",

  "outbounds": [
    "primary",
    "backup"
  ],
  "url": "",
  "interval": "",
  "idle_timeout": "",
  "interrupt_exist_connections": false
}
```

### Fields

#### outbounds

==Required==

Outbound tags in priority order.

#### url

URL used for connectivity check. `https://www.gstatic.com/generate_204` will be used if empty.

#### interval

Check interval. `3m` will be used if empty.

#### idle_timeout

Idle timeout. `30m` will be used if empty.

#### interrupt_exist_connections

Interrupt existing connections when selected outbound changes.

Only inbound connections are affected; internal connections are always interrupted.
