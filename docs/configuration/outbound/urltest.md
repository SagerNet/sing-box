### Structure

```json
{
  "type": "urltest",
  "tag": "auto",
  
  "outbounds": [
    "proxy-a",
    "proxy-b",
    "proxy-c"
  ],
  "select_mode": "",
  "url": "",
  "interval": "",
  "tolerance": 0,
  "idle_timeout": "",
  "interrupt_exist_connections": false
}
```

### Fields

#### outbounds

==Required==

List of outbound tags to test.


#### select_mode

Selection strategy for choosing the active outbound. Defaults to `min_latency` when empty.

- `min_latency`: choose the outbound with the lowest measured delay (respects tolerance).
- `first_available`: choose the first outbound (by configuration order) that is currently healthy (has a recent successful test); if none are healthy, fall back to the first outbound that supports the requested network.


#### url

The URL to test. `https://www.gstatic.com/generate_204` will be used if empty.

#### interval

The test interval. `3m` will be used if empty.

#### tolerance

The test tolerance in milliseconds. `50` will be used if empty. Effective only when `select_mode` is `min_latency`.

#### idle_timeout

The idle timeout. `30m` will be used if empty.

#### interrupt_exist_connections

Interrupt existing connections when the selected outbound has changed.

Only inbound connections are affected by this setting, internal connections will always be interrupted.
