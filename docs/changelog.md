#### 1.0-beta3

* Add [chained inbound](/configuration/shared/listen#detour) support
* Add process_path rule item
* Add macOS redirect support
* Add ShadowTLS [Inbound](/configuration/inbound/shadowtls), [Outbound](/configuration/outbound/shadowtls) and [Examples](/examples/shadowtls)
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
