`leastload` determines the stability or load of nodes based on the standard deviation of the round-trip time from recent checks.

### Structure

```json
{
  "type": "leastload",
  "tag": "leastload-balancer",
  
  "outbounds": [
    "proxy-"
  ],
  "fallback": "block",
  "check": {
    ... // Health Check Fields
  },
  "pick": {
    ... // Balancer Node Pick Fields
  }
}
```

### Fields

#### outbounds

List of outbound tags / tag prefixes.

for example, if there are `proxy-a`, `proxy-b`,`proxy-c` outbounds in the system:

- `proxy-a` will match the specific `proxy-a` outbound.
- `proxy-` will match all the above outbounds.

#### fallback

==Required==

The fallback outbound tag. if no outbound matches the policy, the fallback outbound will be used.

### Health Check Fields

See [Health Check](/configuration/shared/health_check/)。

### Balancer Node Pick Fields

See [Balancer Node Pick](/configuration/shared/node_pick/)。
