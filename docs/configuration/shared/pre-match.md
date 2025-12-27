---
icon: material/new-box
---

# Pre-match

!!! quote "Changes in sing-box 1.13.0"

    :material-plus: [bypass](#bypass)

Pre-match is rule matching that runs before the connection is established.

### How it works

When TUN receives a connection request, the connection has not yet been established,
so no connection data can be read. In this phase, sing-box runs the routing rules in pre-match mode.

Since connection data is unavailable, only actions that do not require connection data can be executed.
When a rule matches an action that requires an established connection, pre-match stops at that rule.

### Supported actions

#### reject

Reject with TCP RST / ICMP unreachable.

See [reject](/configuration/route/rule_action/#reject) for details.

#### route

Route ICMP connections to the specified outbound for direct reply.

See [route](/configuration/route/rule_action/#route) for details.

#### bypass

!!! question "Since sing-box 1.13.0"

!!! quote ""

    Only supported on Linux with `auto_redirect` enabled.

Bypass sing-box and connect directly at kernel level.

If `outbound` is not specified, the rule only matches in pre-match from auto redirect,
and will be skipped in other contexts.

For all other contexts, bypass with `outbound` behaves like `route` action.

See [bypass](/configuration/route/rule_action/#bypass) for details.
