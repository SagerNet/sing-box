# :material-expansion-card: TUN

## :material-text-box: Definition

Refers to TUNnel, a virtual network device supported by the kernel.
It’s also used in sing-box to denote the extensive functionality surrounding TUN inbound:
including traffic assembly, automatic routing, and network and default interface monitoring.

The following flow chart describes the minimal TUN-based transparent proxy process in sing-box:

``` mermaid
flowchart LR
    subgraph inbound [Inbound]
        direction TB
        packet[IP Packet]
        packet --> windows[Windows / macOS]
        packet --> linux[Linux]
        tun[TUN interface]
        windows -. route .-> tun
        linux -. iproute2 route/rule .-> tun
        tun --> gvisor[gVisor TUN stack]
        tun --> system[system TUN stack]
        assemble([L3 to L4 assemble])
        gvisor --> assemble
        system --> assemble
        assemble --> conn[TCP and UDP connections]
        conn --> router[sing-box Router]
    end

    subgraph outbound [Outbound]
        direction TB
        direct[Direct outbound]
        proxy[Proxy outbounds]
        direct --> adi([auto detect interface])
        proxy --> adi
        adi --> default[Default network interface in the system]
        default --> destination[Destination server]
        default --> proxy_server[Proxy server]
        proxy_server --> destination
    end

    inbound --> outbound
```

## :material-help-box: How to

A basic TUN-based transparent proxy configuration file includes: an TUN inbound, `route.auto_detect_interface`, like:

```json
{
  "inbounds": [
    {
      "type": "tun",
      "inet4_address": "172.19.0.1/30",
      "inet6_address": "fdfe:dcba:9876::1/126",
      "auto_route": true,
      "strict_route": true
    }
  ],
  "route": {
    "auto_detect_interface": true
  }
}
```

TODO: finish this wiki