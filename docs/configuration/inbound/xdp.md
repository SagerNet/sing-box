`xdp` inbound captures traffic at the network driver layer using Linux AF_XDP, bypassing the kernel network stack entirely for maximum throughput.

Requires Linux 5.9+ and `with_gvisor` build tag.

### Structure

```json
{
  "type": "xdp",
  "tag": "xdp-in",

  "interface": "eth0",
  "address": [
    "10.0.0.1/24"
  ],
  "route_address": [
    "0.0.0.0/0",
    "::/0"
  ],
  "route_exclude_address": [
    "192.168.0.0/16"
  ],
  "mtu": 1500,
  "frame_size": 4096,
  "frame_count": 4096,
  "udp_timeout": "5m"
}
```

### Fields

#### interface

==Required==

The network interface to attach the XDP program to.

#### address

Local IP address(es) for the internal network stack, with prefix length.

Auto-detected from the interface if omitted.

#### route_address

Destination CIDR prefixes to capture and redirect to sing-box.

Captures all traffic (`0.0.0.0/0` + `::/0`) if empty.

#### route_exclude_address

Destination CIDR prefixes to exclude from capture. Matching packets are passed to the kernel stack.

#### mtu

MTU of the virtual network interface.

Default: `1500`

#### frame_size

AF_XDP UMEM frame size in bytes.

Default: `4096`

#### frame_count

Total number of UMEM frames shared across all RX queues.

Default: `4096`

#### udp_timeout

UDP session timeout.

Default: `5m`
