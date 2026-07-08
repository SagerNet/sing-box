---
icon: material/new-box
---

!!! question "Since sing-box 1.14.0"

# Bridge

!!! quote ""

    Requires privileges. Supported on Linux, macOS, rooted Android, and jailbroken iOS.

    For graphical clients: on macOS, only available in the standalone version and requires the
    Root Helper; on Android, requires root permission; on iOS, requires jailbreak.

`bridge` is the L3 counterpart of the `direct` outbound: it forwards L3 connections
(TCP, UDP and ICMP) directly out of a network interface. Route L3 traffic to it from a TUN
or other L3 endpoints via the `route` action in
[Pre-match](/configuration/shared/pre-match/); L4 connections will be rejected.

Traffic to local addresses of the machine (loopback, or addresses assigned to its
network interfaces) will be rejected.

It is recommended to use [`preferred_by`](/configuration/route/rule/#preferred_by)
as a gate in the `route` rule: it only matches in
[pre-match](/configuration/shared/pre-match/) and excludes local addresses that
cannot be routed.

### Structure

```json
{
  "type": "bridge",
  "tag": "bridge-out",

  "interface": "",
  "bridge_name": "",
  "iproute2_table_index": 0,
  "iproute2_rule_index": 0
}
```

### Fields

#### interface

Interface name for forwarded traffic to egress.

The default interface will be used by default.

Forwarded traffic will be dropped while the interface is unavailable.

#### bridge_name

Custom bridge TUN interface name prefix, `bridge` is used by default.

Not effective on Apple platforms.

#### iproute2_table_index

!!! quote ""

    Only supported on Linux, and only takes effect when `interface` is set.

Linux iproute2 table index for pinned egress routes.

`2200` + instance index is used by default.

#### iproute2_rule_index

!!! quote ""

    Only supported on Linux.

Linux iproute2 rule start index.

`100` is used by default.
