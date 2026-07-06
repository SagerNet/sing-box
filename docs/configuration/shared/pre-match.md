---
icon: material/new-box
---

# Pre-match

!!! quote "Changes in sing-box 1.14.0"

    :material-alert: [route](#route)  
    :material-plus: [sniff](#sniff)

!!! quote "Changes in sing-box 1.13.0"

    :material-plus: [bypass](#bypass)

Pre-match is rule matching that runs before the connection is established.

### How it works

When an L3 inbound (TUN, WireGuard, or Tailscale) receives a connection request, the connection has not yet been established:
for TCP connections no connection data is available, while for UDP connections only the first packet is available.
In this phase, sing-box runs the routing rules in pre-match mode.

When a rule matches an action that requires more connection data than available, pre-match stops at that rule.

### Supported actions

#### reject

Reject with TCP RST / ICMP unreachable.

See [reject](/configuration/route/rule_action/#reject) for details.

#### route

!!! quote "Changes in sing-box 1.14.0"

    Since sing-box 1.14.0, TCP and UDP connections can also be forwarded at L3;
    previously only ICMP connections were supported.

Forward connections directly at L3 to the specified outbound,
without going through L3 to L4 translation.

Supported targets:

- ICMP connections: Direct outbounds and WireGuard / Tailscale endpoints.
- TCP and UDP connections: WireGuard and Tailscale endpoints.

L3 forwarding also applies when no rule matches and the default outbound is a supported
target; for outbound groups, the currently selected outbound is used.

FakeIP destinations require a `resolve` action performed in pre-match,
otherwise connections will be rejected.

See [route](/configuration/route/rule_action/#route) for details.

#### sniff

!!! question "Since sing-box 1.14.0"

For UDP connections, the first packet is available in pre-match,
so protocol sniffing runs on it directly and rule matching continues with the sniffed metadata.

When sniffers require more data (like a fragmented QUIC Client Hello), pre-match stops at that rule.

For TCP connections, pre-match always stops at that rule.

See [sniff](/configuration/route/rule_action/#sniff) for details.

#### bypass

!!! question "Since sing-box 1.13.0"

!!! quote ""

    Only supported on Linux with `auto_redirect` enabled.

Bypass sing-box and connect directly at kernel level.

If `outbound` is not specified, the rule only matches in pre-match from auto redirect,
and will be skipped in other contexts.

For all other contexts, bypass with `outbound` behaves like `route` action.

See [bypass](/configuration/route/rule_action/#bypass) for details.
