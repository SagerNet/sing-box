### Structure

```json
{
  "type": "loadbalance",
  "tag": "auto",

  "outbounds": ["proxy-a", "proxy-b", "proxy-c"],
  "strategy": "round-robin",
  "url": "",
  "interval": "",
  "idle_timeout": ""
}
```

### Fields

#### outbounds

==Required==

List of outbound tags to balance.

#### strategy

Load balancing strategy

| Mode                | Description                                      |
| :------------------ | :----------------------------------------------- |
| `round-robin`       | Cycle through outbounds in order                 |
| `least-connections` | Pick the outbound with fewest active connections |
| `source-hash`       | Hash the client IP to a deterministic outbound   |
| `consistent-hash`   | Consistent hash ring with jump hash              |

`round-robin` is used by default.

#### url

The URL to test. `https://www.gstatic.com/generate_204` will be used if empty.

#### interval

The test interval. `3m` will be used if empty.

#### idle_timeout

The idle timeout. `30m` will be used if empty.
