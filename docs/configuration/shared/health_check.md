### Structure

```json
{
  "interval": "60s",
  "sampling": 10,
  "timeout": "5s",
  "destination": "http://www.gstatic.com/generate_204",
  "connectivity": "http://connectivitycheck.platform.hicloud.com/generate_204",
  "max_rtt": "1000ms",
  "tolerance": 0.2
}
```

### Fields

#### interval

The interval of health check for each node. Must be greater than `10s`, default is `60s`.

#### sampling

The number of recent health check results to sample. Must be greater than `0`, default is `10`.

#### timeout

The timeout of each health check. Must be greater than `0s`, default is `5s`.

#### destination

The destination of health check. Default is `http://www.gstatic.com/generate_204`.

#### connectivity

The destination of connectivity check. If health check fails, it may be caused by network unavailability (e.g. disconnecting from WIFI). Set this field to avoid the node being judged to be invalid under such circumstances. If not set, this behavior will not occur.

#### max_rtt

The maximum round-trip time of health check that is acceptable. Nodes that exceed this value will not be selected. Default is `0`, which accepts any node.

#### tolerance

The tolerance of health check failure. Must be greater than `0` and less than `1`, default is `0`.

`sampling=10, tolerance=0.2` means that in the last 10 checks, 2 failures are allowed at most.
