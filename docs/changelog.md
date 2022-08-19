#### 2022/08/19

* Add Hysteria [Inbound](/configuration/inbound/hysteria) and [Outbund](/configuration/outbound/hysteria)
* Add [ACME TLS certificate issuer](/configuration/shared/tls)
* Allow read config from stdin (-c stdin)
* Update gVisor to 20220815.0

#### 2022/08/18

* Fix find process with lwip stack
* Fix crash on shadowsocks server
* Fix crash on darwin tun
* Fix write log to file

#### 2022/08/17

* Improve async dns transports

#### 2022/08/16

* Add ip_version (route/dns) rule item
* Add [WireGuard](/configuration/outbound/wireguard) outbound

#### 2022/08/15

* Add uid, android user and package rules support in [Tun](/configuration/inbound/tun) routing.

#### 2022/08/13

* Fix dns concurrent write

#### 2022/08/12

* Performance improvements
* Add UoT option for [Socks](/configuration/outbound/socks) outbound

#### 2022/08/11

* Add UoT option for [Shadowsocks](/configuration/outbound/shadowsocks) outbound, UoT support for all inbounds

#### 2022/08/10

* Add full-featured [Naive](/configuration/inbound/naive) inbound
* Fix default dns server option [#9] by iKirby

#### 2022/08/09

No changelog before.

[#9]: https://github.com/SagerNet/sing-box/pull/9