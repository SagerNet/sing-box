---
icon: material/lan
---

# Neighbor Resolution

Match LAN devices by MAC address and hostname using
[`source_mac_address`](/configuration/route/rule/#source_mac_address) and
[`source_hostname`](/configuration/route/rule/#source_hostname) rule items.

Neighbor resolution is automatically enabled when these rule items exist.
Use [`route.find_neighbor`](/configuration/route/#find_neighbor) to force enable it for logging without rules.

## Linux

Works natively. No special setup required.

Hostname resolution requires DHCP lease files,
automatically detected from common DHCP servers (dnsmasq, odhcpd, ISC dhcpd, Kea).
Custom paths can be set via [`route.dhcp_lease_files`](/configuration/route/#dhcp_lease_files).

## Android

!!! quote ""

    Only supported in graphical clients.

Requires Android 11 or above and ROOT.

Must use [VPNHotspot](https://github.com/Mygod/VPNHotspot) to share the VPN connection.
ROM built-in features like "Use VPN for connected devices" can share VPN
but cannot provide MAC address or hostname information.

Set **IP Masquerade Mode** to **None** in VPNHotspot settings.

Only route/DNS rules are supported. TUN include/exclude routes are not supported.

### Hostname Visibility

Hostname is only visible in sing-box if it is visible in VPNHotspot.
For Apple devices, change **Private Wi-Fi Address** from **Rotating** to **Fixed** in the Wi-Fi settings
of the connected network. Non-Apple devices are always visible.

## macOS

Requires the standalone version (macOS system extension).
The App Store version can share the VPN as a hotspot but does not support MAC address or hostname reading.

See [VPN Hotspot](/manual/misc/vpn-hotspot/#macos) for Internet Sharing setup.
