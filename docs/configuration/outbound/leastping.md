`leastping` determines the response speed of nodes based on the average round-trip time of recent checks. The nodes selected by this strategy are often those are closer to the server of check destination.

### Structure

```json
{
  "type": "leastping",
  "tag": "leastping-balancer",
  
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
