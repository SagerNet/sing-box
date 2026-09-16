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
  "url": "",
  "interval": "",
  "tolerance": 0,
  "idle_timeout": "",
  "interrupt_exist_connections": false,
  "bandwidth_test": {
    "enabled": false,
    "url": "",
    "max_bytes": 0,
    "timeout": "",
    "interval": "",
    "concurrency": 0,
    "strategy": "",
    "latency_floor": "",
    "throughput_tolerance": 0,
    "samples": 0
  }
}
```

### Fields

#### outbounds

==Required==

List of outbound tags to test.

#### url

The URL to test. `https://www.gstatic.com/generate_204` will be used if empty.

#### interval

The test interval. `3m` will be used if empty.

#### tolerance

The test tolerance in milliseconds. `50` will be used if empty.

#### idle_timeout

The idle timeout. `30m` will be used if empty.

#### interrupt_exist_connections

Interrupt existing connections when the selected outbound has changed.

Only inbound connections are affected by this setting, internal connections will always be interrupted.

#### bandwidth_test

!!! question "Since sing-box 1.14.0"

Optional bandwidth-aware probing, disabled by default.

The latency probe measures the time to response headers, which says nothing about sustained
throughput. On a congested or shaped path the two decouple: a small probe can finish quickly on a
path whose throughput has collapsed, because it completes before the connection leaves slow start.
When this is enabled, each outbound is additionally probed with a bounded `GET`, and the effective
transfer rate becomes available as a selection input.

When disabled, the probe path is unchanged and no response body is ever transferred.

!!! warning "This is not a speed test"

    The measurement is deliberately bounded, and reads only a few hundred KiB — while the flow is
    still in or near slow start. The absolute rate understates a fast path's real capacity, and must
    not be presented to users as a speed test result. It is a *ranking* signal: the ratio between a
    shaped and an unshaped path is already large at this scale. Shaping that only engages after
    several MiB will not be detected.

!!! warning "Data usage"

    Unlike the latency probe, this transfers a payload over every outbound on every interval. At the
    default 256 KiB with 10 outbounds every 15 minutes, that is roughly 10 MiB/hour, which matters
    on a metered connection. Probing inherits the group's idle suspension and pause handling, so it
    stops while the group is unused or the device is asleep.

#### bandwidth_test.enabled

Enable bandwidth probing.

#### bandwidth_test.url

The URL to download from. It must return a body of at least `max_bytes`; `generate_204` returns no
body and cannot be used.

`https://speed.cloudflare.com/__down?bytes=<max_bytes>` will be used if empty — the byte count
tracks `max_bytes`, so the response is exactly as large as the probe reads.

!!! warning "Prefer your own endpoint"

    The default is shared by every client that enables this feature. If you operate a suitable
    endpoint, or your provider does, point at that instead.

The URL must return `2xx` directly. Redirects are not followed, so a redirecting endpoint fails the
probe — use the final URL. Note that the latency probe accepts any status while this one requires
`2xx`, so an endpoint can pass latency probing and still report zero throughput.

#### bandwidth_test.max_bytes

A hard cap on the body bytes read per probe. `262144` (256 KiB) will be used if empty; values above
`1048576` (1 MiB) are rejected.

This caps bytes *read*, not bytes *retained* — the body is read into one small reusable buffer and
discarded, so memory use is independent of this setting.

#### bandwidth_test.timeout

Per-probe timeout. `5s` will be used if empty.

A probe that times out short of the cap still yields a sample, computed from the bytes actually
transferred: a timeout is itself evidence of a slow path.

#### bandwidth_test.interval

The bandwidth test interval. Five times `interval` will be used if empty.

The right cadence for a throughput probe is much lower than for a liveness probe. Setting this
higher than `idle_timeout` means probing will rarely run.

#### bandwidth_test.concurrency

How many bandwidth probes may run at once. `2` will be used if empty.

Deliberately far below the latency sweep's fixed 10: these probes consume bandwidth, so running many
at once makes them contend with each other and skews every result.

#### bandwidth_test.strategy

How the selection is ranked:

| Strategy                        | Behavior                                                                                             |
| ------------------------------- | ---------------------------------------------------------------------------------------------------- |
| `latency`                       | Default. Rank by latency, unchanged. Throughput is measured and exposed, but not used for selection. |
| `throughput`                    | Rank by throughput. Latency is ignored beyond liveness.                                              |
| `throughput_with_latency_floor` | Discard outbounds whose latency exceeds `latency_floor`, then rank the survivors by throughput.      |

Selection falls back to latency ranking whenever no outbound has a throughput sample yet — during
startup, before the first bandwidth sweep, and after a network change.

#### bandwidth_test.latency_floor

Under `throughput_with_latency_floor`, outbounds whose latency exceeds this are excluded from
throughput ranking. Empty or `0` disables the floor, which makes that strategy equivalent to
`throughput`.

This keeps an outbound that is pathologically slow to connect from winning on bulk transfer alone,
which matters for interactive traffic.

#### bandwidth_test.throughput_tolerance

Relative hysteresis, as a percentage. `25` will be used if empty.

A challenger only replaces the current outbound when its throughput exceeds it by this margin. The
band is relative rather than absolute because throughput ratios are the meaningful comparison. This
is the throughput counterpart of `tolerance`.

#### bandwidth_test.samples

How many recent samples to smooth over, using the median. `3` will be used if empty.

Throughput samples are noisier than latency samples — a probe landing during a transient burst can
swing the value severalfold. Smoothing keeps the group from oscillating and repeatedly interrupting
connections. A failed probe is recorded as a zero sample, so a sustained failure decays an outbound
out of contention while a single transient one is absorbed.
