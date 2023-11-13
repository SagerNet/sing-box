#### 1.6.4

* Fixes and improvements

#### 1.6.3

* iOS/Android: Fix profile auto update
* Fixes and improvements

#### 1.6.2

* Fixes and improvements

#### 1.6.1

* Our [Android client](/installation/clients/sfa) is now available in the Google Play Store ▶️
* Fixes and improvements

#### 1.6.0

* Fixes and improvements

Important changes since 1.5:

* Our [Apple tvOS client](/installation/clients/sft) is now available in the App Store 🍎
* Update BBR congestion control for TUIC and Hysteria2 **1**
* Update brutal congestion control for Hysteria2
* Add `brutal_debug` option for Hysteria2
* Update legacy Hysteria protocol **2**
* Add TLS self sign key pair generate command
* Remove [Deprecated Features](/deprecated) by agreement

**1**:

None of the existing Golang BBR congestion control implementations have been reviewed or unit tested.
This update is intended to address the multi-send defects of the old implementation and may introduce new issues.

**2**

Based on discussions with the original author, the brutal CC and QUIC protocol parameters of
the old protocol (Hysteria 1) have been updated to be consistent with Hysteria 2


#### 1.5.5

* Fix IPv6 `auto_route` for Linux **1**
* Add legacy builds for old Windows and macOS systems **2**
* Fixes and improvements

**1**:

When `auto_route` is enabled and `strict_route` is disabled, the device can now be reached from external IPv6 addresses.

**2**:

Built using Go 1.20, the last version that will run on Windows 7, 8, Server 2008, Server 2012 and macOS 10.13 High Sierra, 10.14 Mojave.


#### 1.6.0-rc.4

* Fixes and improvements

#### 1.6.0-rc.1

* Add legacy builds for old Windows and macOS systems **1**
* Fixes and improvements

**1**:

Built using Go 1.20, the last version that will run on Windows 7, 8, Server 2008, Server 2012 and macOS 10.13 High Sierra, 10.14 Mojave.

#### 1.6.0-beta.4

* Fix IPv6 `auto_route` for Linux **1**
* Fixes and improvements

**1**:

When `auto_route` is enabled and `strict_route` is disabled, the device can now be reached from external IPv6 addresses.

#### 1.5.4

* Fix Clash cache crash on arm32 devices
* Fixes and improvements

#### 1.6.0-beta.3

* Update the legacy Hysteria protocol **1**
* Fixes and improvements

**1**

Based on discussions with the original author, the brutal CC and QUIC protocol parameters of
the old protocol (Hysteria 1) have been updated to be consistent with Hysteria 2

#### 1.6.0-beta.2

* Add TLS self sign key pair generate command
* Update brutal congestion control for Hysteria2
* Fix Clash cache crash on arm32 devices
* Update golang.org/x/net to v0.17.0
* Fixes and improvements

#### 1.5.3

* Fix compatibility with Android 14
* Fixes and improvements

#### 1.6.0-beta.1

* Fixes and improvements

#### 1.6.0-alpha.5

* Fix compatibility with Android 14
* Update BBR congestion control for TUIC and Hysteria2 **1**
* Fixes and improvements

**1**:

None of the existing Golang BBR congestion control implementations have been reviewed or unit tested.
This update is intended to fix a memory leak flaw in the new implementation introduced in 1.6.0-alpha.1 and may
introduce new issues.

#### 1.6.0-alpha.4

* Add `brutal_debug` option for Hysteria2
* Fixes and improvements

#### 1.5.2

* Our [Apple tvOS client](/installation/clients/sft) is now available in the App Store 🍎
* Fixes and improvements

#### 1.6.0-alpha.3

* Fixes and improvements

#### 1.6.0-alpha.2

* Fixes and improvements

#### 1.5.1

* Fixes and improvements

#### 1.6.0-alpha.1

* Update BBR congestion control for TUIC and Hysteria2 **1**
* Update quic-go to v0.39.0
* Update gVisor to 20230814.0
* Remove [Deprecated Features](/deprecated) by agreement
* Fixes and improvements

**1**:

None of the existing Golang BBR congestion control implementations have been reviewed or unit tested.
This update is intended to address the multi-send defects of the old implementation and may introduce new issues.

#### 1.5.0

* Fixes and improvements

Important changes since 1.4:

* Add TLS [ECH server](/configuration/shared/tls) support
* Improve TLS TCH client configuration
* Add TLS ECH key pair generator **1**
* Add TLS ECH support for QUIC based protocols **2**
* Add KDE support for the `set_system_proxy` option in HTTP inbound
* Add Hysteria2 protocol support **3**
* Add `interrupt_exist_connections` option for `Selector` and `URLTest` outbounds **4**
* Add DNS01 challenge support for ACME TLS certificate issuer **5**
* Add `merge` command **6**
* Mark [Deprecated Features](/deprecated)

**1**:

Command: `sing-box generate ech-keypair <plain_server_name> [--pq-signature-schemes-enabled]`

**2**:

All inbounds and outbounds are supported, including `Naiveproxy`, `Hysteria[/2]`, `TUIC` and `V2ray QUIC transport`.

**3**:

See [Hysteria2 inbound](/configuration/inbound/hysteria2) and [Hysteria2 outbound](/configuration/outbound/hysteria2)

For protocol description, please refer to [https://v2.hysteria.network](https://v2.hysteria.network)

**4**:

Interrupt existing connections when the selected outbound has changed.

Only inbound connections are affected by this setting, internal connections will always be interrupted.

**5**:

Only `Alibaba Cloud DNS` and `Cloudflare` are supported, see [ACME Fields](/configuration/shared/tls#acme-fields)
and [DNS01 Challenge Fields](/configuration/shared/dns01_challenge).

**6**:

This command also parses path resources that appear in the configuration file and replaces them with embedded
configuration, such as TLS certificates or SSH private keys.

#### 1.5.0-rc.6

* Fixes and improvements

#### 1.4.6

* Fixes and improvements

#### 1.5.0-rc.5

* Fixed an improper authentication vulnerability in the SOCKS5 inbound
* Fixes and improvements

**Security Advisory**

This update fixes an improper authentication vulnerability in the sing-box SOCKS inbound. This vulnerability allows an
attacker to craft special requests to bypass user authentication. All users exposing SOCKS servers with user
authentication in an insecure environment are advised to update immediately.

此更新修复了 sing-box SOCKS 入站中的一个不正确身份验证漏洞。 该漏洞允许攻击者制作特殊请求来绕过用户身份验证。建议所有将使用用户认证的
SOCKS 服务器暴露在不安全环境下的用户立更新。

#### 1.4.5

* Fixed an improper authentication vulnerability in the SOCKS5 inbound
* Fixes and improvements

**Security Advisory**

This update fixes an improper authentication vulnerability in the sing-box SOCKS inbound. This vulnerability allows an
attacker to craft special requests to bypass user authentication. All users exposing SOCKS servers with user
authentication in an insecure environment are advised to update immediately.

此更新修复了 sing-box SOCKS 入站中的一个不正确身份验证漏洞。 该漏洞允许攻击者制作特殊请求来绕过用户身份验证。建议所有将使用用户认证的
SOCKS 服务器暴露在不安全环境下的用户立更新。

#### 1.5.0-rc.3

* Fixes and improvements

#### 1.5.0-beta.12

* Add `merge` command **1**
* Fixes and improvements

**1**:

This command also parses path resources that appear in the configuration file and replaces them with embedded
configuration, such as TLS certificates or SSH private keys.

```
Merge configurations

Usage:
  sing-box merge [output] [flags]

Flags:
  -h, --help   help for merge

Global Flags:
  -c, --config stringArray             set configuration file path
  -C, --config-directory stringArray   set configuration directory path
  -D, --directory string               set working directory
      --disable-color                  disable color output
```

#### 1.5.0-beta.11

* Add DNS01 challenge support for ACME TLS certificate issuer **1**
* Fixes and improvements

**1**:

Only `Alibaba Cloud DNS` and `Cloudflare` are supported,
see [ACME Fields](/configuration/shared/tls#acme-fields)
and [DNS01 Challenge Fields](/configuration/shared/dns01_challenge).

#### 1.5.0-beta.10

* Add `interrupt_exist_connections` option for `Selector` and `URLTest` outbounds **1**
* Fixes and improvements

**1**:

Interrupt existing connections when the selected outbound has changed.

Only inbound connections are affected by this setting, internal connections will always be interrupted.

#### 1.4.3

* Fixes and improvements

#### 1.5.0-beta.8

* Fixes and improvements

#### 1.4.2

* Fixes and improvements

#### 1.5.0-beta.6

* Fix compatibility issues with official Hysteria2 server and client
* Fixes and improvements
* Mark [deprecated features](/deprecated)

#### 1.5.0-beta.3

* Fixes and improvements
* Updated Hysteria2 documentation **1**

**1**:

Added notes indicating compatibility issues with the official
Hysteria2 server and client when using `fastOpen=false` or UDP MTU >= 1200.

#### 1.5.0-beta.2

* Add hysteria2 protocol support **1**
* Fixes and improvements

**1**:

See [Hysteria2 inbound](/configuration/inbound/hysteria2) and [Hysteria2 outbound](/configuration/outbound/hysteria2)

For protocol description, please refer to [https://v2.hysteria.network](https://v2.hysteria.network)

#### 1.5.0-beta.1

* Add TLS [ECH server](/configuration/shared/tls) support
* Improve TLS TCH client configuration
* Add TLS ECH key pair generator **1**
* Add TLS ECH support for QUIC based protocols **2**
* Add KDE support for the `set_system_proxy` option in HTTP inbound

**1**:

Command: `sing-box generate ech-keypair <plain_server_name> [--pq-signature-schemes-enabled]`

**2**:

All inbounds and outbounds are supported, including `Naiveproxy`, `Hysteria`, `TUIC` and `V2ray QUIC transport`.

#### 1.4.1

* Fixes and improvements

#### 1.4.0

* Fix bugs and update dependencies

Important changes since 1.3:

* Add TUIC support **1**
* Add `udp_over_stream` option for TUIC client **2**
* Add MultiPath TCP support **3**
* Add `include_interface` and `exclude_interface` options for tun inbound
* Pause recurring tasks when no network or device idle
* Improve Android and Apple platform clients

*1*:

See [TUIC inbound](/configuration/inbound/tuic)
and [TUIC outbound](/configuration/outbound/tuic)

**2**:

This is the TUIC port of the [UDP over TCP protocol](/configuration/shared/udp-over-tcp), designed to provide a QUIC
stream based UDP relay mode that TUIC does not provide. Since it is an add-on protocol, you will need to use sing-box or
another program compatible with the protocol as a server.

This mode has no positive effect in a proper UDP proxy scenario and should only be applied to relay streaming UDP
traffic (basically QUIC streams).

*3*:

Requires sing-box to be compiled with Go 1.21.

#### 1.4.0-rc.3

* Fixes and improvements

#### 1.4.0-rc.2

* Fixes and improvements

#### 1.4.0-rc.1

* Fix TUIC UDP

#### 1.4.0-beta.6

* Add `udp_over_stream` option for TUIC client **1**
* Add `include_interface` and `exclude_interface` options for tun inbound
* Fixes and improvements

**1**:

This is the TUIC port of the [UDP over TCP protocol](/configuration/shared/udp-over-tcp), designed to provide a QUIC
stream based UDP relay mode that TUIC does not provide. Since it is an add-on protocol, you will need to use sing-box or
another program compatible with the protocol as a server.

This mode has no positive effect in a proper UDP proxy scenario and should only be applied to relay streaming UDP
traffic (basically QUIC streams).

#### 1.4.0-beta.5

* Fixes and improvements

#### 1.4.0-beta.4

* Graphical clients: Persistence group expansion state
* Fixes and improvements

#### 1.4.0-beta.3

* Fixes and improvements

#### 1.4.0-beta.2

* Add MultiPath TCP support **1**
* Drop QUIC support for Go 1.18 and 1.19 due to upstream changes
* Fixes and improvements

*1*:

Requires sing-box to be compiled with Go 1.21.

#### 1.4.0-beta.1

* Add TUIC support **1**
* Pause recurring tasks when no network or device idle
* Fixes and improvements

*1*:

See [TUIC inbound](/configuration/inbound/tuic)
and [TUIC outbound](/configuration/outbound/tuic)

#### 1.3.6

* Fixes and improvements

#### 1.3.5

* Fixes and improvements
* Introducing our [Apple tvOS](/installation/clients/sft) client applications **1**
* Add per app proxy and app installed/updated trigger support for Android client
* Add profile sharing support for Android/iOS/macOS clients

**1**:

Due to the requirement of tvOS 17, the app cannot be submitted to the App Store for the time being, and can only be
downloaded through TestFlight.

#### 1.3.4

* Fixes and improvements
* We're now on the [App Store](https://apps.apple.com/us/app/sing-box/id6451272673), always free! It should be noted
  that due to stricter and slower review, the release of Store versions will be delayed.
* We've made a standalone version of the macOS client (the original Application Extension relies on App Store
  distribution), which you can download as SFM-version-universal.zip in the release artifacts.

#### 1.3.3

* Fixes and improvements

#### 1.3.1-rc.1

* Fix bugs and update dependencies

#### 1.3.1-beta.3

* Introducing our [new iOS](/installation/clients/sfi) and [macOS](/installation/clients/sfm) client applications **1**
* Fixes and improvements

**1**:

The old testflight link and app are no longer valid.

#### 1.3.1-beta.2

* Fix bugs and update dependencies

#### 1.3.1-beta.1

* Fixes and improvements

#### 1.3.0

* Fix bugs and update dependencies

Important changes since 1.2:

* Add [FakeIP](/configuration/dns/fakeip) support **1**
* Improve multiplex **2**
* Add [DNS reverse mapping](/configuration/dns#reverse_mapping) support
* Add `rewrite_ttl` DNS rule action
* Add `store_fakeip` Clash API option
* Add multi-peer support for [WireGuard](/configuration/outbound/wireguard#peers) outbound
* Add loopback detect
* Add Clash.Meta API compatibility for Clash API
* Download Yacd-meta by default if the specified Clash `external_ui` directory is empty
* Add path and headers option for HTTP outbound
* Perform URLTest recheck after network changes
* Fix `system` tun stack for ios
* Fix network monitor for android/ios
* Update VLESS and XUDP protocol
* Make splice work with traffic statistics systems like Clash API
* Significantly reduces memory usage of idle connections
* Improve DNS caching
* Add `independent_cache` [option](/configuration/dns#independent_cache) for DNS
* Reimplemented shadowsocks client
* Add multiplex support for VLESS outbound
* Automatically add Windows firewall rules in order for the system tun stack to work
* Fix TLS 1.2 support for shadow-tls client
* Add `cache_id` [option](/configuration/experimental#cache_id) for Clash cache file
* Fix `local` DNS transport for Android

*1*:

See [FAQ](/faq/fakeip) for more information.

*2*:

Added new `h2mux` multiplex protocol and `padding` multiplex option, see [Multiplex](/configuration/shared/multiplex).

#### 1.3-rc2

* Fix `local` DNS transport for Android
* Fix bugs and update dependencies

#### 1.3-rc1

* Fix bugs and update dependencies

#### 1.3-beta14

* Fixes and improvements

#### 1.3-beta13

* Fix resolving fakeip domains  **1**
* Deprecate L3 routing
* Fix bugs and update dependencies

**1**:

If the destination address of the connection is obtained from fakeip, dns rules with server type fakeip will be skipped.

#### 1.3-beta12

* Automatically add Windows firewall rules in order for the system tun stack to work
* Fix TLS 1.2 support for shadow-tls client
* Add `cache_id` [option](/configuration/experimental#cache_id) for Clash cache file
* Fixes and improvements

#### 1.3-beta11

* Fix bugs and update dependencies

#### 1.3-beta10

* Improve direct copy **1**
* Improve DNS caching
* Add `independent_cache` [option](/configuration/dns#independent_cache) for DNS
* Reimplemented shadowsocks client **2**
* Add multiplex support for VLESS outbound
* Set TCP keepalive for WireGuard gVisor TCP connections
* Fixes and improvements

**1**:

* Make splice work with traffic statistics systems like Clash API
* Significantly reduces memory usage of idle connections

**2**:

Improved performance and reduced memory usage.

#### 1.3-beta9

* Improve multiplex **1**
* Fixes and improvements

*1*:

Added new `h2mux` multiplex protocol and `padding` multiplex option, see [Multiplex](/configuration/shared/multiplex).

#### 1.2.6

* Fix bugs and update dependencies

#### 1.3-beta8

* Fix `system` tun stack for ios
* Fix network monitor for android/ios
* Update VLESS and XUDP protocol **1**
* Fixes and improvements

*1:

This is an incompatible update for XUDP in VLESS if vision flow is enabled.

#### 1.3-beta7

* Add `path` and `headers` options for HTTP outbound
* Add multi-user support for Shadowsocks legacy AEAD inbound
* Fixes and improvements

#### 1.2.4

* Fixes and improvements

#### 1.3-beta6

* Fix WireGuard reconnect
* Perform URLTest recheck after network changes
* Fix bugs and update dependencies

#### 1.3-beta5

* Add Clash.Meta API compatibility for Clash API
* Download Yacd-meta by default if the specified Clash `external_ui` directory is empty
* Add path and headers option for HTTP outbound
* Fixes and improvements

#### 1.3-beta4

* Fix bugs

#### 1.3-beta2

* Download clash-dashboard if the specified Clash `external_ui` directory is empty
* Fix bugs and update dependencies

#### 1.3-beta1

* Add [DNS reverse mapping](/configuration/dns#reverse_mapping) support
* Add [L3 routing](/configuration/route/ip-rule) support **1**
* Add `rewrite_ttl` DNS rule action
* Add [FakeIP](/configuration/dns/fakeip) support **2**
* Add `store_fakeip` Clash API option
* Add multi-peer support for [WireGuard](/configuration/outbound/wireguard#peers) outbound
* Add loopback detect

*1*:

It can currently be used to [route connections directly to WireGuard](/examples/wireguard-direct) or block connections
at the IP layer.

*2*:

See [FAQ](/faq/fakeip) for more information.

#### 1.2.3

* Introducing our [new Android client application](/installation/clients/sfa)
* Improve UDP domain destination NAT
* Update reality protocol
* Fix TTL calculation for DNS response
* Fix v2ray HTTP transport compatibility
* Fix bugs and update dependencies

#### 1.2.2

* Accept `any` outbound in dns rule **1**
* Fix bugs and update dependencies

*1*:

Now you can use the `any` outbound rule to match server address queries instead of filling in all server domains
to `domain` rule.

#### 1.2.1

* Fix missing default host in v2ray http transport`s request
* Flush DNS cache for macOS when tun start/close
* Fix tun's DNS hijacking compatibility with systemd-resolved

#### 1.2.0

* Fix bugs and update dependencies

Important changes since 1.1:

* Introducing our [new iOS client application](/installation/clients/sfi)
* Introducing [UDP over TCP protocol version 2](/configuration/shared/udp-over-tcp)
* Add [platform options](/configuration/inbound/tun#platform) for tun inbound
* Add [ShadowTLS protocol v3](https://github.com/ihciah/shadow-tls/blob/master/docs/protocol-v3-en.md)
* Add [VLESS server](/configuration/inbound/vless) and [vision](/configuration/outbound/vless#flow) support
* Add [reality TLS](/configuration/shared/tls) support
* Add [NTP service](/configuration/ntp)
* Add [DHCP DNS server](/configuration/dns/server) support
* Add SSH [host key validation](/configuration/outbound/ssh) support
* Add [query_type](/configuration/dns/rule) DNS rule item
* Add fallback support for v2ray transport
* Add custom TLS server support for http based v2ray transports
* Add health check support for http-based v2ray transports
* Add multiple configuration support

#### 1.2-rc1

* Fix bugs and update dependencies

#### 1.2-beta10

* Add multiple configuration support **1**
* Fix bugs and update dependencies

*1*:

Now you can pass the parameter `--config` or `-c` multiple times, or use the new parameter `--config-directory` or `-C`
to load all configuration files in a directory.

Loaded configuration files are sorted by name. If you want to control the merge order, add a numeric prefix to the file
name.

#### 1.1.7

* Improve the stability of the VMESS server
* Fix `auto_detect_interface` incorrectly identifying the default interface on Windows
* Fix bugs and update dependencies

#### 1.2-beta9

* Introducing the [UDP over TCP protocol version 2](/configuration/shared/udp-over-tcp)
* Add health check support for http-based v2ray transports
* Remove length limit on short_id for reality TLS config
* Fix bugs and update dependencies

#### 1.2-beta8

* Update reality and uTLS libraries
* Fix `auto_detect_interface` incorrectly identifying the default interface on Windows

#### 1.2-beta7

* Fix the compatibility issue between VLESS's vision sub-protocol and the Xray-core client
* Improve the stability of the VMESS server

#### 1.2-beta6

* Introducing our [new iOS client application](/installation/clients/sfi)
* Add [platform options](/configuration/inbound/tun#platform) for tun inbound
* Add custom TLS server support for http based v2ray transports
* Add generate commands
* Enable XUDP by default in VLESS
* Update reality server
* Update vision protocol
* Fixed [user flow in vless server](/configuration/inbound/vless#usersflow)
* Bug fixes
* Update dependencies

#### 1.2-beta5

* Add [VLESS server](/configuration/inbound/vless) and [vision](/configuration/outbound/vless#flow) support
* Add [reality TLS](/configuration/shared/tls) support
* Fix match private address

#### 1.1.6

* Improve vmess request
* Fix ipv6 redirect on Linux
* Fix match geoip private
* Fix parse hysteria UDP message
* Fix socks connect response
* Disable vmess header protection if transport enabled
* Update QUIC v2 version number and initial salt

#### 1.2-beta4

* Add [NTP service](/configuration/ntp)
* Add Add multiple server names and multi-user support for shadowtls
* Add strict mode support for shadowtls v3
* Add uTLS support for shadowtls v3

#### 1.2-beta3

* Update QUIC v2 version number and initial salt
* Fix shadowtls v3 implementation

#### 1.2-beta2

* Add [ShadowTLS protocol v3](https://github.com/ihciah/shadow-tls/blob/master/docs/protocol-v3-en.md)
* Add fallback support for v2ray transport
* Fix parse hysteria UDP message
* Fix socks connect response
* Disable vmess header protection if transport enabled

#### 1.2-beta1

* Add [DHCP DNS server](/configuration/dns/server) support
* Add SSH [host key validation](/configuration/outbound/ssh) support
* Add [query_type](/configuration/dns/rule) DNS rule item
* Add v2ray [user stats](/configuration/experimental#statsusers) api
* Add new clash DNS query api
* Improve vmess request
* Fix ipv6 redirect on Linux
* Fix match geoip private

#### 1.1.5

* Add Go 1.20 support
* Fix inbound default DF value
* Fix auth_user route for naive inbound
* Fix gRPC lite header
* Ignore domain case in route rules

#### 1.1.4

* Fix DNS log
* Fix write to h2 conn after closed
* Fix create UDP DNS transport from plain IPv6 address

#### 1.1.2

* Fix http proxy auth
* Fix user from stream packet conn
* Fix DNS response TTL
* Fix override packet conn
* Skip override system proxy bypass list
* Improve DNS log

#### 1.1.1

* Fix acme config
* Fix vmess packet conn
* Suppress quic-go set DF error

#### 1.1

* Fix close clash cache

Important changes since 1.0:

* Add support for use with android VPNService
* Add tun support for WireGuard outbound
* Add system tun stack
* Add comment filter for config
* Add option for allow optional proxy protocol header
* Add Clash mode and persistence support
* Add TLS ECH and uTLS support for outbound TLS options
* Add internal simple-obfs and v2ray-plugin
* Add ShadowsocksR outbound
* Add VLESS outbound and XUDP
* Skip wait for hysteria tcp handshake response
* Add v2ray mux support for all inbound
* Add XUDP support for VMess
* Improve websocket writer
* Refine tproxy write back
* Fix DNS leak caused by
  Windows' ordinary multihomed DNS resolution behavior
* Add sniff_timeout listen option
* Add custom route support for tun
* Add option for custom wireguard reserved bytes
* Split bind_address into ipv4 and ipv6
* Add ShadowTLS v1 and v2 support

#### 1.1-rc1

* Fix TLS config for h2 server
* Fix crash when input bad method in shadowsocks multi-user inbound
* Fix listen UDP
* Fix check invalid packet on macOS

#### 1.1-beta18

* Enhance defense against active probe for shadowtls server **1**

**1**:

The `fallback_after` option has been removed.

#### 1.1-beta17

* Fix shadowtls server **1**

*1*:

Added [fallback_after](/configuration/inbound/shadowtls#fallback_after) option.

#### 1.0.7

* Add support for new x/h2 deadline
* Fix copy pipe
* Fix decrypt xplus packet
* Fix macOS Ventura process name match
* Fix smux keepalive
* Fix vmess request buffer
* Fix h2c transport
* Fix tor geoip
* Fix udp connect for mux client
* Fix default dns transport strategy

#### 1.1-beta16

* Improve shadowtls server
* Fix default dns transport strategy
* Update uTLS to v1.2.0

#### 1.1-beta15

* Add support for new x/h2 deadline
* Fix udp connect for mux client
* Fix dns buffer
* Fix quic dns retry
* Fix create TLS config
* Fix websocket alpn
* Fix tor geoip

#### 1.1-beta14

* Add multi-user support for hysteria inbound **1**
* Add custom tls client support for std grpc
* Fix smux keep alive
* Fix vmess request buffer
* Fix default local DNS server behavior
* Fix h2c transport

*1*:

The `auth` and `auth_str` fields have been replaced by the `users` field.

#### 1.1-beta13

* Add custom worker count option for WireGuard outbound
* Split bind_address into ipv4 and ipv6
* Move WFP manipulation to strict route
* Fix WireGuard outbound panic when close
* Fix macOS Ventura process name match
* Fix QUIC connection migration by @HyNetwork
* Fix handling QUIC client SNI by @HyNetwork

#### 1.1-beta12

* Fix uTLS config
* Update quic-go to v0.30.0
* Update cloudflare-tls to go1.18.7

#### 1.1-beta11

* Add option for custom wireguard reserved bytes
* Fix shadowtls v2
* Fix h3 dns transport
* Fix copy pipe
* Fix decrypt xplus packet
* Fix v2ray api
* Suppress no network error
* Improve local dns transport

#### 1.1-beta10

* Add [sniff_timeout](/configuration/shared/listen#sniff_timeout) listen option
* Add [custom route](/configuration/inbound/tun#inet4_route_address) support for tun **1**
* Fix interface monitor
* Fix websocket headroom
* Fix uTLS handshake
* Fix ssh outbound
* Fix sniff fragmented quic client hello
* Fix DF for hysteria
* Fix naive overflow
* Check destination before udp connect
* Update uTLS to v1.1.5
* Update tfo-go to v2.0.2
* Update fsnotify to v1.6.0
* Update grpc to v1.50.1

*1*:

The `strict_route` on windows is removed.

#### 1.0.6

* Fix ssh outbound
* Fix sniff fragmented quic client hello
* Fix naive overflow
* Check destination before udp connect

#### 1.1-beta9

* Fix windows route **1**
* Add [v2ray statistics api](/configuration/experimental#v2ray-api-fields)
* Add ShadowTLS v2 support **2**
* Fixes and improvements

**1**:

* Fix DNS leak caused by
  Windows' [ordinary multihomed DNS resolution behavior](https://learn.microsoft.com/en-us/previous-versions/windows/it-pro/windows-server-2008-R2-and-2008/dd197552%28v%3Dws.10%29)
* Flush Windows DNS cache when start/close

**2**:

See [ShadowTLS inbound](/configuration/inbound/shadowtls#version)
and [ShadowTLS outbound](/configuration/outbound/shadowtls#version)

#### 1.1-beta8

* Fix leaks on close
* Improve websocket writer
* Refine tproxy write back
* Refine 4in6 processing
* Fix shadowsocks plugins
* Fix missing source address from transport connection
* Fix fqdn socks5 outbound connection
* Fix read source address from grpc-go

#### 1.0.5

* Fix missing source address from transport connection
* Fix fqdn socks5 outbound connection
* Fix read source address from grpc-go

#### 1.1-beta7

* Add v2ray mux and XUDP support for VMess inbound
* Add XUDP support for VMess outbound
* Disable DF on direct outbound by default
* Fix bugs in 1.1-beta6

#### 1.1-beta6

* Add [URLTest outbound](/configuration/outbound/urltest)
* Fix bugs in 1.1-beta5

#### 1.1-beta5

* Print tags in version command
* Redirect clash hello to external ui
* Move shadowsocksr implementation to clash
* Make gVisor optional **1**
* Refactor to miekg/dns
* Refactor bind control
* Fix build on go1.18
* Fix clash store-selected
* Fix close grpc conn
* Fix port rule match logic
* Fix clash api proxy type

*1*:

The build tag `no_gvisor` is replaced by `with_gvisor`.

The default tun stack is changed to system.

#### 1.0.4

* Fix close grpc conn
* Fix port rule match logic
* Fix clash api proxy type

#### 1.1-beta4

* Add internal simple-obfs and v2ray-plugin [Shadowsocks plugins](/configuration/outbound/shadowsocks#plugin)
* Add [ShadowsocksR outbound](/configuration/outbound/shadowsocksr)
* Add [VLESS outbound and XUDP](/configuration/outbound/vless)
* Skip wait for hysteria tcp handshake response
* Fix socks4 client
* Fix hysteria inbound
* Fix concurrent write

#### 1.0.3

* Fix socks4 client
* Fix hysteria inbound
* Fix concurrent write

#### 1.1-beta3

* Fix using custom TLS client in http2 client
* Fix bugs in 1.1-beta2

#### 1.1-beta2

* Add Clash mode and persistence support **1**
* Add TLS ECH and uTLS support for outbound TLS options **2**
* Fix socks4 request
* Fix processing empty dns result

*1*:

Switching modes using the Clash API, and `store-selected` are now supported,
see [Experimental](/configuration/experimental).

*2*:

ECH (Encrypted Client Hello) is a TLS extension that allows a client to encrypt the first part of its ClientHello
message, see [TLS#ECH](/configuration/shared/tls#ech).

uTLS is a fork of "crypto/tls", which provides ClientHello fingerprinting resistance,
see [TLS#uTLS](/configuration/shared/tls#utls).

#### 1.0.2

* Fix socks4 request
* Fix processing empty dns result

#### 1.1-beta1

* Add support for use with android VPNService **1**
* Add tun support for WireGuard outbound **2**
* Add system tun stack **3**
* Add comment filter for config **4**
* Add option for allow optional proxy protocol header
* Add half close for smux
* Set UDP DF by default **5**
* Set default tun mtu to 9000
* Update gVisor to 20220905.0

*1*:

In previous versions, Android VPN would not work with tun enabled.

The usage of tun over VPN and VPN over tun is now supported, see [Tun Inbound](/configuration/inbound/tun#auto_route).

*2*:

In previous releases, WireGuard outbound support was backed by the lower performance gVisor virtual interface.

It achieves the same performance as wireguard-go by providing automatic system interface support.

*3*:

It does not depend on gVisor and has better performance in some cases.

It is less compatible and may not be available in some environments.

*4*:

Annotated json configuration files are now supported.

*5*:

UDP fragmentation is now blocked by default.

Including shadowsocks-libev, shadowsocks-rust and quic-go all disable segmentation by default.

See [Dial Fields](/configuration/shared/dial#udp_fragment)
and [Listen Fields](/configuration/shared/listen#udp_fragment).

#### 1.0.1

* Fix match 4in6 address in ip_cidr
* Fix clash api log level format error
* Fix clash api unknown proxy type

#### 1.0

* Fix wireguard reconnect
* Fix naive inbound
* Fix json format error message
* Fix processing vmess termination signal
* Fix hysteria stream error
* Fix listener close when proxyproto failed

#### 1.0-rc1

* Fix write log timestamp
* Fix write zero
* Fix dial parallel in direct outbound
* Fix write trojan udp
* Fix DNS routing
* Add attribute support for geosite
* Update documentation for [Dial Fields](/configuration/shared/dial)

#### 1.0-beta3

* Add [chained inbound](/configuration/shared/listen#detour) support
* Add process_path rule item
* Add macOS redirect support
* Add ShadowTLS [Inbound](/configuration/inbound/shadowtls), [Outbound](/configuration/outbound/shadowtls)
  and [Examples](/examples/shadowtls)
* Fix search android package in non-owner users
* Fix socksaddr type condition
* Fix smux session status
* Refactor inbound and outbound documentation
* Minor fixes

#### 1.0-beta2

* Add strict_route option for [Tun inbound](/configuration/inbound/tun#strict_route)
* Add packetaddr support for [VMess outbound](/configuration/outbound/vmess#packet_addr)
* Add better performing alternative gRPC implementation
* Add [docker image](https://github.com/SagerNet/sing-box/pkgs/container/sing-box)
* Fix sniff override destination

#### 1.0-beta1

* Initial release

##### 2022/08/26

* Fix ipv6 route on linux
* Fix read DNS message

##### 2022/08/25

* Let vmess use zero instead of auto if TLS enabled
* Add trojan fallback for ALPN
* Improve ip_cidr rule
* Fix format bind_address
* Fix http proxy with compressed response
* Fix route connections

##### 2022/08/24

* Fix naive padding
* Fix unix search path
* Fix close non-duplex connections
* Add ACME EAB support
* Fix early close on windows and catch any
* Initial zh-CN document translation

##### 2022/08/23

* Add [V2Ray Transport](/configuration/shared/v2ray-transport) support for VMess and Trojan
* Allow plain http request in Naive inbound (It can now be used with nginx)
* Add proxy protocol support
* Free memory after start
* Parse X-Forward-For in HTTP requests
* Handle SIGHUP signal

##### 2022/08/22

* Add strategy setting for each [DNS server](/configuration/dns/server)
* Add bind address to outbound options

##### 2022/08/21

* Add [Tor outbound](/configuration/outbound/tor)
* Add [SSH outbound](/configuration/outbound/ssh)

##### 2022/08/20

* Attempt to unwrap ip-in-fqdn socksaddr
* Fix read packages in android 12
* Fix route on some android devices
* Improve linux process searcher
* Fix write socks5 username password auth request
* Skip bind connection with private destination to interface
* Add [Trojan connection fallback](/configuration/inbound/trojan#fallback)

##### 2022/08/19

* Add Hysteria [Inbound](/configuration/inbound/hysteria) and [Outbund](/configuration/outbound/hysteria)
* Add [ACME TLS certificate issuer](/configuration/shared/tls)
* Allow read config from stdin (-c stdin)
* Update gVisor to 20220815.0

##### 2022/08/18

* Fix find process with lwip stack
* Fix crash on shadowsocks server
* Fix crash on darwin tun
* Fix write log to file

##### 2022/08/17

* Improve async dns transports

##### 2022/08/16

* Add ip_version (route/dns) rule item
* Add [WireGuard](/configuration/outbound/wireguard) outbound

##### 2022/08/15

* Add uid, android user and package rules support in [Tun](/configuration/inbound/tun) routing.

##### 2022/08/13

* Fix dns concurrent write

##### 2022/08/12

* Performance improvements
* Add UoT option for [SOCKS](/configuration/outbound/socks) outbound

##### 2022/08/11

* Add UoT option for [Shadowsocks](/configuration/outbound/shadowsocks) outbound, UoT support for all inbounds

##### 2022/08/10

* Add full-featured [Naive](/configuration/inbound/naive) inbound
* Fix default dns server option [#9] by iKirby

##### 2022/08/09

No changelog before.

[#9]: https://github.com/SagerNet/sing-box/pull/9
