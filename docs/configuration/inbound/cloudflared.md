---
icon: material/new-box
---

!!! question "Since sing-box 1.14.0"

`cloudflared` inbound runs an embedded Cloudflare Tunnel client and routes all
incoming tunnel traffic (TCP, UDP, ICMP) through sing-box's routing engine.

### Structure

```json
{
  "type": "cloudflared",
  "tag": "",

  "token": "",
  "ha_connections": 0,
  "protocol": "",
  "post_quantum": false,
  "edge_ip_version": 0,
  "datagram_version": "",
  "grace_period": "",
  "region": "",
  "control_dialer": {
    ... // Dial Fields
  },
  "tunnel_dialer": {
    ... // Dial Fields
  }
}
```

### Fields

#### token

==Required==

Base64-encoded tunnel token from the Cloudflare Zero Trust dashboard
(`Networks → Tunnels → Install connector`).

#### ha_connections

Number of high-availability connections to the Cloudflare edge.

Capped by the number of discovered edge addresses.

#### protocol

Transport protocol for edge connections.

One of `quic` `http2`.

#### post_quantum

Enable post-quantum key exchange on the control connection.

#### edge_ip_version

IP version used when connecting to the Cloudflare edge.

One of `0` (automatic) `4` `6`.

#### datagram_version

Datagram protocol version used for UDP proxying over QUIC.

One of `v2` `v3`. Only meaningful when `protocol` is `quic`.

#### grace_period

Graceful shutdown window for in-flight edge connections.

#### region

Cloudflare edge region selector.

Conflict with endpoints embedded in `token`.

#### control_dialer

[Dial Fields](/configuration/shared/dial/) used when the tunnel client dials the
Cloudflare control plane.

#### tunnel_dialer

[Dial Fields](/configuration/shared/dial/) used when the tunnel client dials the
Cloudflare edge data plane.
