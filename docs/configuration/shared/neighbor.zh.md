---
icon: material/lan
---

# 邻居解析

通过
[`source_mac_address`](/configuration/route/rule/#source_mac_address) 和
[`source_hostname`](/configuration/route/rule/#source_hostname) 规则项匹配局域网设备的 MAC 地址和主机名。

当这些规则项存在时，邻居解析自动启用。
使用 [`route.find_neighbor`](/configuration/route/#find_neighbor) 可在没有规则时强制启用以输出日志。

## Linux

原生支持，无需特殊设置。

主机名解析需要 DHCP 租约文件，
自动从常见 DHCP 服务器（dnsmasq、odhcpd、ISC dhcpd、Kea）检测。
可通过 [`route.dhcp_lease_files`](/configuration/route/#dhcp_lease_files) 设置自定义路径。

## Android

!!! quote ""

    仅在图形客户端中支持。

需要 Android 11 或以上版本和 ROOT。

必须使用 [VPNHotspot](https://github.com/Mygod/VPNHotspot) 共享 VPN 连接。
ROM 自带的「通过 VPN 共享连接」等功能可以共享 VPN，
但无法提供 MAC 地址或主机名信息。

在 VPNHotspot 设置中将 **IP 遮掩模式** 设为 **无**。

仅支持路由/DNS 规则。不支持 TUN 的 include/exclude 路由。

### 设备可见性

MAC 地址和主机名仅在 VPNHotspot 中可见时 sing-box 才能读取。
对于 Apple 设备，需要在所连接网络的 Wi-Fi 设置中将**私有无线局域网地址**从**轮替**改为**固定**。
非 Apple 设备始终可见。

## macOS

需要独立版本（macOS 系统扩展）。
App Store 版本可以共享 VPN 热点但不支持 MAC 地址或主机名读取。

参阅 [VPN 热点](/manual/misc/vpn-hotspot/#macos) 了解互联网共享设置。
