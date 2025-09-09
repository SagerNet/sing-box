---
icon: material/alert-decagram
---

#### 1.13.0-alpha.26

* Update quic-go to v0.55.0
* Fix memory leak in hysteria2
* Fixes and improvements

#### 1.12.11

* Fixes and improvements

#### 1.13.0-alpha.24

* Add Claude Code Multiplexer service **1**
* Fixes and improvements

**1**:

CCM (Claude Code Multiplexer) service allows you to access your local Claude Code subscription remotely through custom tokens, eliminating the need for OAuth authentication on remote clients.

See [CCM](/configuration/service/ccm).

#### 1.13.0-alpha.23

* Fix compatibility with MPTCP **1**
* Fixes and improvements

**1**:

`auto_redirect` now rejects MPTCP connections by default to fix compatibility issues,
but you can change it to bypass the sing-box via the new `exclude_mptcp` option.

See [TUN](/configuration/inbound/tun/#exclude_mptcp).

#### 1.13.0-alpha.22

* Update uTLS to v1.8.1 **1**
* Fixes and improvements

**1**:

This update fixes an critical issue that could cause simulated Chrome fingerprints to be detected,
see https://github.com/refraction-networking/utls/pull/375.

#### 1.12.10

* Update uTLS to v1.8.1 **1**
* Fixes and improvements

**1**:

This update fixes an critical issue that could cause simulated Chrome fingerprints to be detected,
see https://github.com/refraction-networking/utls/pull/375.

#### 1.13.0-alpha.21

* Fix missing mTLS support in client options **1**
* Fixes and improvements

See [TLS](/configuration/shared/tls/).

#### 1.12.9

* Fixes and improvements

#### 1.13.0-alpha.16

* Add curve preferences, pinned public key SHA256 and mTLS for TLS options **1**
* Fixes and improvements

See [TLS](/configuration/shared/tls/).

#### 1.13.0-alpha.15

* Update quic-go to v0.54.0
* Update gVisor to v20250811
* Update Tailscale to v1.86.5
* Fixes and improvements

#### 1.12.8

* Fixes and improvements

#### 1.13.0-alpha.11

* Fixes and improvements

#### 1.12.5

* Fixes and improvements

#### 1.13.0-alpha.10

* Improve kTLS support **1**
* Fixes and improvements

**1**:

kTLS is now compatible with custom TLS implementations other than uTLS.

#### 1.12.4

* Fixes and improvements

#### 1.12.3

* Fixes and improvements

#### 1.12.2

* Fixes and improvements

#### 1.12.1

* Fixes and improvements

#### 1.12.0

* Refactor DNS servers **1**
* Add domain resolver options**2**
* Add TLS fragment/record fragment support to route options and outbound TLS options **3**
* Add certificate options **4**
* Add Tailscale endpoint and DNS server **5**
* Drop support for go1.22 **6**
* Add AnyTLS protocol **7**
* Migrate to stdlib ECH implementation **8**
* Add NTP sniffer **9**
* Add wildcard SNI support for ShadowTLS inbound **10**
* Improve `auto_redirect` **11**
* Add control options for listeners **12**
* Add DERP service **13**
* Add Resolved service and DNS server **14**
* Add SSM API service **15**
* Add loopback address support for tun **16**
* Improve tun performance on Apple platforms **17**
* Update quic-go to v0.52.0
* Update gVisor to 20250319.0
* Update the status of graphical clients in stores **18**

**1**:

DNS servers are refactored for better performance and scalability.

See [DNS server](/configuration/dns/server/).

For migration, see [Migrate to new DNS server formats](/migration/#migrate-to-new-dns-servers).

Compatibility for old formats will be removed in sing-box 1.14.0.

**2**:

Legacy `outbound` DNS rules are deprecated
and can be replaced by the new `domain_resolver` option.

See [Dial Fields](/configuration/shared/dial/#domain_resolver) and
[Route](/configuration/route/#default_domain_resolver).

For migration,
see [Migrate outbound DNS rule items to domain resolver](/migration/#migrate-outbound-dns-rule-items-to-domain-resolver).

**3**:

See [Route Action](/configuration/route/rule_action/#tls_fragment) and [TLS](/configuration/shared/tls/).

**4**:

New certificate options allow you to manage the default list of trusted X509 CA certificates.

For the system certificate list, fixed Go not reading Android trusted certificates correctly.

You can also use the Mozilla Included List instead, or add trusted certificates yourself.

See [Certificate](/configuration/certificate/).

**5**:

See [Tailscale](/configuration/endpoint/tailscale/).

**6**:

Due to maintenance difficulties, sing-box 1.12.0 requires at least Go 1.23 to compile.

For Windows 7 users, legacy binaries now continue to compile with Go 1.23 and patches
from [MetaCubeX/go](https://github.com/MetaCubeX/go).

**7**:

The new AnyTLS protocol claims to mitigate TLS proxy traffic characteristics and comes with a new multiplexing scheme.

See [AnyTLS Inbound](/configuration/inbound/anytls/) and [AnyTLS Outbound](/configuration/outbound/anytls/).

**8**:

See [TLS](/configuration/shared/tls).

The build tag `with_ech` is no longer needed and has been removed.

**9**:

See [Protocol Sniff](/configuration/route/sniff/).

**10**:

See [ShadowTLS](/configuration/inbound/shadowtls/#wildcard_sni).

**11**:

Now `auto_redirect` fixes compatibility issues between tun and Docker bridge networks,
see [Tun](/configuration/inbound/tun/#auto_redirect).

**12**:

You can now set `bind_interface`, `routing_mark` and `reuse_addr` in Listen Fields.

See [Listen Fields](/configuration/shared/listen/).

**13**:

DERP service is a Tailscale DERP server, similar to [derper](https://pkg.go.dev/tailscale.com/cmd/derper).

See [DERP Service](/configuration/service/derp/).

**14**:

Resolved service is a fake systemd-resolved DBUS service to receive DNS settings from other programs
(e.g. NetworkManager) and provide DNS resolution.

See [Resolved Service](/configuration/service/resolved/) and [Resolved DNS Server](/configuration/dns/server/resolved/).

**15**:

SSM API service is a RESTful API server for managing Shadowsocks servers.

See [SSM API Service](/configuration/service/ssm-api/).

**16**:

TUN now implements SideStore's StosVPN.

See [Tun](/configuration/inbound/tun/#loopback_address).

**17**:

We have significantly improved the performance of tun inbound on Apple platforms, especially in the gVisor stack.

The following data was tested
using [tun_bench](https://github.com/SagerNet/sing-box/blob/dev-next/cmd/internal/tun_bench/main.go) on M4 MacBook pro.

| Version     | Stack  | MTU   | Upload | Download |
|-------------|--------|-------|--------|----------|
| 1.11.15     | gvisor | 1500  | 852M   | 2.57G    |
| 1.12.0-rc.4 | gvisor | 1500  | 2.90G  | 4.68G    |
| 1.11.15     | gvisor | 4064  | 2.31G  | 6.34G    |
| 1.12.0-rc.4 | gvisor | 4064  | 7.54G  | 12.2G    |
| 1.11.15     | gvisor | 65535 | 27.6G  | 18.1G    |
| 1.12.0-rc.4 | gvisor | 65535 | 39.8G  | 34.7G    |
| 1.11.15     | system | 1500  | 664M   | 706M     |
| 1.12.0-rc.4 | system | 1500  | 2.44G  | 2.51G    |
| 1.11.15     | system | 4064  | 1.88G  | 1.94G    |
| 1.12.0-rc.4 | system | 4064  | 6.45G  | 6.27G    |
| 1.11.15     | system | 65535 | 26.2G  | 17.4G    |
| 1.12.0-rc.4 | system | 65535 | 17.6G  | 21.0G    |

**18**:

We continue to experience issues updating our sing-box apps on the App Store and Play Store.
Until we rewrite and resubmit the apps, they are considered irrecoverable.
Therefore, after this release, we will not be repeating this notice unless there is new information.

### 1.11.15

* Fixes and improvements

_We are temporarily unable to update sing-box apps on the App Store because the reviewer mistakenly found that we
violated the rules (TestFlight users are not affected)._

#### 1.12.0-beta.32

* Improve tun performance on Apple platforms **1**
* Fixes and improvements

**1**:

We have significantly improved the performance of tun inbound on Apple platforms, especially in the gVisor stack.

### 1.11.14

* Fixes and improvements

_We are temporarily unable to update sing-box apps on the App Store because the reviewer mistakenly found that we
violated the rules (TestFlight users are not affected)._

#### 1.12.0-beta.24

* Allow `tls_fragment` and `tls_record_fragment` to be enabled together **1**
* Also add fragment options for TLS client configuration **2**
* Fixes and improvements

**1**:

For debugging only, it is recommended to disable if record fragmentation works.

See [Route Action](/configuration/route/rule_action/#tls_fragment).

**2**:

See [TLS](/configuration/shared/tls/).

#### 1.12.0-beta.23

* Add loopback address support for tun **1**
* Add cache support for ssm-api **2**
* Fixes and improvements

**1**:

TUN now implements SideStore's StosVPN.

See [Tun](/configuration/inbound/tun/#loopback_address).

**2**:

See [SSM API Service](/configuration/service/ssm-api/#cache_path).

#### 1.12.0-beta.21

* Fix missing `home` option for DERP service **1**
* Fixes and improvements

**1**:

You can now choose what the DERP home page shows, just like with derper's `-home` flag.

See [DERP](/configuration/service/derp/#home).

### 1.11.13

* Fixes and improvements

_We are temporarily unable to update sing-box apps on the App Store because the reviewer mistakenly found that we
violated the rules (TestFlight users are not affected)._

#### 1.12.0-beta.17

* Update quic-go to v0.52.0
* Fixes and improvements

#### 1.12.0-beta.15

* Add DERP service **1**
* Add Resolved service and DNS server **2**
* Add SSM API service **3**
* Fixes and improvements

**1**:

DERP service is a Tailscale DERP server, similar to [derper](https://pkg.go.dev/tailscale.com/cmd/derper).

See [DERP Service](/configuration/service/derp/).

**2**:

Resolved service is a fake systemd-resolved DBUS service to receive DNS settings from other programs
(e.g. NetworkManager) and provide DNS resolution.

See [Resolved Service](/configuration/service/resolved/) and [Resolved DNS Server](/configuration/dns/server/resolved/).

**3**:

SSM API service is a RESTful API server for managing Shadowsocks servers.

See [SSM API Service](/configuration/service/ssm-api/).

### 1.11.11

* Fixes and improvements

_We are temporarily unable to update sing-box apps on the App Store because the reviewer mistakenly found that we
violated the rules (TestFlight users are not affected)._

#### 1.12.0-beta.13

* Add TLS record fragment route options **1**
* Add missing `accept_routes` option for Tailscale **2**
* Fixes and improvements

**1**:

See [Route Action](/configuration/route/rule_action/#tls_record_fragment).

**2**:

See [Tailscale](/configuration/endpoint/tailscale/#accept_routes).

#### 1.12.0-beta.10

* Add control options for listeners **1**
* Fixes and improvements

**1**:

You can now set `bind_interface`, `routing_mark` and `reuse_addr` in Listen Fields.

See [Listen Fields](/configuration/shared/listen/).

### 1.11.10

* Undeprecate the `block` outbound **1**
* Fixes and improvements

**1**:

Since we don’t have a replacement for using the `block` outbound in selectors yet,
we decided to temporarily undeprecate the `block` outbound until a replacement is available in the future.

_We are temporarily unable to update sing-box apps on the App Store because the reviewer mistakenly found that we
violated the rules (TestFlight users are not affected)._

#### 1.12.0-beta.9

* Update quic-go to v0.51.0
* Fixes and improvements

### 1.11.9

* Fixes and improvements

_We are temporarily unable to update sing-box apps on the App Store because the reviewer mistakenly found that we
violated the rules (TestFlight users are not affected)._

#### 1.12.0-beta.5

* Fixes and improvements

### 1.11.8

* Improve `auto_redirect` **1**
* Fixes and improvements

**1**:

Now `auto_redirect` fixes compatibility issues between TUN and Docker bridge networks,
see [Tun](/configuration/inbound/tun/#auto_redirect).

_We are temporarily unable to update sing-box apps on the App Store because the reviewer mistakenly found that we
violated the rules (TestFlight users are not affected)._

#### 1.12.0-beta.3

* Fixes and improvements

### 1.11.7

* Fixes and improvements

_We are temporarily unable to update sing-box apps on the App Store because the reviewer mistakenly found that we
violated the rules (TestFlight users are not affected)._

#### 1.12.0-beta.1

* Fixes and improvements

**1**:

Now `auto_redirect` fixes compatibility issues between tun and Docker bridge networks,
see [Tun](/configuration/inbound/tun/#auto_redirect).

### 1.11.6

* Fixes and improvements

_We are temporarily unable to update sing-box apps on the App Store because the reviewer mistakenly found that we
violated the rules (TestFlight users are not affected)._

#### 1.12.0-alpha.19

* Update gVisor to 20250319.0
* Fixes and improvements

#### 1.12.0-alpha.18

* Add wildcard SNI support for ShadowTLS inbound **1**
* Fixes and improvements

**1**:

See [ShadowTLS](/configuration/inbound/shadowtls/#wildcard_sni).

#### 1.12.0-alpha.17

* Add NTP sniffer **1**
* Fixes and improvements

**1**:

See [Protocol Sniff](/configuration/route/sniff/).

#### 1.12.0-alpha.16

* Update `domain_resolver` behavior **1**
* Fixes and improvements

**1**:

`route.default_domain_resolver` or `outbound.domain_resolver` is now optional when only one DNS server is configured.

See [Dial Fields](/configuration/shared/dial/#domain_resolver).

### 1.11.5

* Fixes and improvements

_We are temporarily unable to update sing-box apps on the App Store because the reviewer mistakenly found that we
violated the rules (TestFlight users are not affected)._

#### 1.12.0-alpha.13

* Move `predefined` DNS server to DNS rule action **1**
* Fixes and improvements

**1**:

See [DNS Rule Action](/configuration/dns/rule_action/#predefined).

### 1.11.4

* Fixes and improvements

#### 1.12.0-alpha.11

* Fixes and improvements

#### 1.12.0-alpha.10

* Add AnyTLS protocol **1**
* Improve `resolve` route action **2**
* Migrate to stdlib ECH implementation **3**
* Fixes and improvements

**1**:

The new AnyTLS protocol claims to mitigate TLS proxy traffic characteristics and comes with a new multiplexing scheme.

See [AnyTLS Inbound](/configuration/inbound/anytls/) and [AnyTLS Outbound](/configuration/outbound/anytls/).

**2**:

`resolve` route action now accepts `disable_cache` and other options like in DNS route actions,
see [Route Action](/configuration/route/rule_action).

**3**:

See [TLS](/configuration/shared/tls).

The build tag `with_ech` is no longer needed and has been removed.

#### 1.12.0-alpha.7

* Add Tailscale DNS server **1**
* Fixes and improvements

**1**:

See [Tailscale](/configuration/dns/server/tailscale/).

#### 1.12.0-alpha.6

* Add Tailscale endpoint **1**
* Drop support for go1.22 **2**
* Fixes and improvements

**1**:

See [Tailscale](/configuration/endpoint/tailscale/).

**2**:

Due to maintenance difficulties, sing-box 1.12.0 requires at least Go 1.23 to compile.

For Windows 7 users, legacy binaries now continue to compile with Go 1.23 and patches
from [MetaCubeX/go](https://github.com/MetaCubeX/go).

### 1.11.3

* Fixes and improvements

_This version overwrites 1.11.2, as incorrect binaries were released due to a bug in the continuous integration
process._

#### 1.12.0-alpha.5

* Fixes and improvements

### 1.11.1

* Fixes and improvements

#### 1.12.0-alpha.2

* Update quic-go to v0.49.0
* Fixes and improvements

#### 1.12.0-alpha.1

* Refactor DNS servers **1**
* Add domain resolver options**2**
* Add TLS fragment route options **3**
* Add certificate options **4**

**1**:

DNS servers are refactored for better performance and scalability.

See [DNS server](/configuration/dns/server/).

For migration, see [Migrate to new DNS server formats](/migration/#migrate-to-new-dns-servers).

Compatibility for old formats will be removed in sing-box 1.14.0.

**2**:

Legacy `outbound` DNS rules are deprecated
and can be replaced by the new `domain_resolver` option.

See [Dial Fields](/configuration/shared/dial/#domain_resolver) and
[Route](/configuration/route/#default_domain_resolver).

For migration,
see [Migrate outbound DNS rule items to domain resolver](/migration/#migrate-outbound-dns-rule-items-to-domain-resolver).

**3**:

The new TLS fragment route options allow you to fragment TLS handshakes to bypass firewalls.

This feature is intended to circumvent simple firewalls based on **plaintext packet matching**, and should not be used
to circumvent real censorship.

Since it is not designed for performance, it should not be applied to all connections, but only to server names that are
known to be blocked.

See [Route Action](/configuration/route/rule_action/#tls_fragment).

**4**:

New certificate options allow you to manage the default list of trusted X509 CA certificates.

For the system certificate list, fixed Go not reading Android trusted certificates correctly.

You can also use the Mozilla Included List instead, or add trusted certificates yourself.

See [Certificate](/configuration/certificate/).

### 1.11.0

Important changes since 1.10:

* Introducing rule actions **1**
* Improve tun compatibility **3**
* Merge route options to route actions **4**
* Add `network_type`, `network_is_expensive` and `network_is_constrainted` rule items **5**
* Add multi network dialing **6**
* Add `cache_capacity` DNS option **7**
* Add `override_address` and `override_port` route options **8**
* Upgrade WireGuard outbound to endpoint **9**
* Add UDP GSO support for WireGuard
* Make GSO adaptive **10**
* Add UDP timeout route option **11**
* Add more masquerade options for hysteria2 **12**
* Add `rule-set merge` command
* Add port hopping support for Hysteria2 **13**
* Hysteria2 `ignore_client_bandwidth` behavior update **14**

**1**:

New rule actions replace legacy inbound fields and special outbound fields,
and can be used for pre-matching **2**.

See [Rule](/configuration/route/rule/),
[Rule Action](/configuration/route/rule_action/),
[DNS Rule](/configuration/dns/rule/) and
[DNS Rule Action](/configuration/dns/rule_action/).

For migration, see
[Migrate legacy special outbounds to rule actions](/migration/#migrate-legacy-special-outbounds-to-rule-actions),
[Migrate legacy inbound fields to rule actions](/migration/#migrate-legacy-inbound-fields-to-rule-actions)
and [Migrate legacy DNS route options to rule actions](/migration/#migrate-legacy-dns-route-options-to-rule-actions).

**2**:

Similar to Surge's pre-matching.

Specifically, new rule actions allow you to reject connections with
TCP RST (for TCP connections) and ICMP port unreachable (for UDP packets)
before connection established to improve tun's compatibility.

See [Rule Action](/configuration/route/rule_action/).

**3**:

When `gvisor` tun stack is enabled, even if the request passes routing,
if the outbound connection establishment fails,
the connection still does not need to be established and a TCP RST is replied.

**4**:

Route options in DNS route actions will no longer be considered deprecated,
see [DNS Route Action](/configuration/dns/rule_action/).

Also, now `udp_disable_domain_unmapping` and `udp_connect` can also be configured in route action,
see [Route Action](/configuration/route/rule_action/).

**5**:

When using in graphical clients, new routing rule items allow you to match on
network type (WIFI, cellular, etc.), whether the network is expensive, and whether Low Data Mode is enabled.

See [Route Rule](/configuration/route/rule/), [DNS Route Rule](/configuration/dns/rule/)
and [Headless Rule](/configuration/rule-set/headless-rule/).

**6**:

Similar to Surge's strategy.

New options allow you to connect using multiple network interfaces,
prefer or only use one type of interface,
and configure a timeout to fallback to other interfaces.

See [Dial Fields](/configuration/shared/dial/#network_strategy),
[Rule Action](/configuration/route/rule_action/#network_strategy)
and [Route](/configuration/route/#default_network_strategy).

**7**:

See [DNS](/configuration/dns/#cache_capacity).

**8**:

See [Rule Action](/configuration/route/#override_address) and
[Migrate destination override fields to route options](/migration/#migrate-destination-override-fields-to-route-options).

**9**:

The new WireGuard endpoint combines inbound and outbound capabilities,
and the old outbound will be removed in sing-box 1.13.0.

See [Endpoint](/configuration/endpoint/), [WireGuard Endpoint](/configuration/endpoint/wireguard/)
and [Migrate WireGuard outbound fields to route options](/migration/#migrate-wireguard-outbound-to-endpoint).

**10**:

For WireGuard outbound and endpoint, GSO will be automatically enabled when available,
see [WireGuard Outbound](/configuration/outbound/wireguard/#gso).

For TUN, GSO has been removed,
see [Deprecated](/deprecated/#gso-option-in-tun).

**11**:

See [Rule Action](/configuration/route/rule_action/#udp_timeout).

**12**:

See [Hysteria2](/configuration/inbound/hysteria2/#masquerade).

**13**:

See [Hysteria2](/configuration/outbound/hysteria2/).

**14**:

When `up_mbps` and `down_mbps` are set, `ignore_client_bandwidth` instead denies clients from using BBR CC.

### 1.10.7

* Fixes and improvements

#### 1.11.0-beta.20

* Hysteria2 `ignore_client_bandwidth` behavior update **1**
* Fixes and improvements

**1**:

When `up_mbps` and `down_mbps` are set, `ignore_client_bandwidth` instead denies clients from using BBR CC.

See [Hysteria2](/configuration/inbound/hysteria2/#ignore_client_bandwidth).

#### 1.11.0-beta.17

* Add port hopping support for Hysteria2 **1**
* Fixes and improvements

**1**:

See [Hysteria2](/configuration/outbound/hysteria2/).

#### 1.11.0-beta.14

* Allow adding route (exclude) address sets to routes **1**
* Fixes and improvements

**1**:

When `auto_redirect` is not enabled, directly add `route[_exclude]_address_set`
to tun routes (equivalent to `route[_exclude]_address`).

Note that it **doesn't work on the Android graphical client** due to
the Android VpnService not being able to handle a large number of routes (DeadSystemException),
but otherwise it works fine on all command line clients and Apple platforms.

See [route_address_set](/configuration/inbound/tun/#route_address_set) and
[route_exclude_address_set](/configuration/inbound/tun/#route_exclude_address_set).

#### 1.11.0-beta.12

* Add `rule-set merge` command
* Fixes and improvements

#### 1.11.0-beta.3

* Add more masquerade options for hysteria2 **1**
* Fixes and improvements

**1**:

See [Hysteria2](/configuration/inbound/hysteria2/#masquerade).

#### 1.11.0-alpha.25

* Update quic-go to v0.48.2
* Fixes and improvements

#### 1.11.0-alpha.22

* Add UDP timeout route option **1**
* Fixes and improvements

**1**:

See [Rule Action](/configuration/route/rule_action/#udp_timeout).

#### 1.11.0-alpha.20

* Add UDP GSO support for WireGuard
* Make GSO adaptive **1**

**1**:

For WireGuard outbound and endpoint, GSO will be automatically enabled when available,
see [WireGuard Outbound](/configuration/outbound/wireguard/#gso).

For TUN, GSO has been removed,
see [Deprecated](/deprecated/#gso-option-in-tun).

#### 1.11.0-alpha.19

* Upgrade WireGuard outbound to endpoint **1**
* Fixes and improvements

**1**:

The new WireGuard endpoint combines inbound and outbound capabilities,
and the old outbound will be removed in sing-box 1.13.0.

See [Endpoint](/configuration/endpoint/), [WireGuard Endpoint](/configuration/endpoint/wireguard/)
and [Migrate WireGuard outbound fields to route options](/migration/#migrate-wireguard-outbound-to-endpoint).

### 1.10.2

* Add deprecated warnings
* Fix proxying websocket connections in HTTP/mixed inbounds
* Fixes and improvements

#### 1.11.0-alpha.18

* Fixes and improvements

#### 1.11.0-alpha.16

* Add `cache_capacity` DNS option **1**
* Add `override_address` and `override_port` route options **2**
* Fixes and improvements

**1**:

See [DNS](/configuration/dns/#cache_capacity).

**2**:

See [Rule Action](/configuration/route/#override_address) and
[Migrate destination override fields to route options](/migration/#migrate-destination-override-fields-to-route-options).

#### 1.11.0-alpha.15

* Improve multi network dialing **1**
* Fixes and improvements

**1**:

New options allow you to configure the network strategy flexibly.

See [Dial Fields](/configuration/shared/dial/#network_strategy),
[Rule Action](/configuration/route/rule_action/#network_strategy)
and [Route](/configuration/route/#default_network_strategy).

#### 1.11.0-alpha.14

* Add multi network dialing **1**
* Fixes and improvements

**1**:

Similar to Surge's strategy.

New options allow you to connect using multiple network interfaces,
prefer or only use one type of interface,
and configure a timeout to fallback to other interfaces.

See [Dial Fields](/configuration/shared/dial/#network_strategy),
[Rule Action](/configuration/route/rule_action/#network_strategy)
and [Route](/configuration/route/#default_network_strategy).

#### 1.11.0-alpha.13

* Fixes and improvements

#### 1.11.0-alpha.12

* Merge route options to route actions **1**
* Add `network_type`, `network_is_expensive` and `network_is_constrainted` rule items **2**
* Fixes and improvements

**1**:

Route options in DNS route actions will no longer be considered deprecated,
see [DNS Route Action](/configuration/dns/rule_action/).

Also, now `udp_disable_domain_unmapping` and `udp_connect` can also be configured in route action,
see [Route Action](/configuration/route/rule_action/).

**2**:

When using in graphical clients, new routing rule items allow you to match on
network type (WIFI, cellular, etc.), whether the network is expensive, and whether Low Data Mode is enabled.

See [Route Rule](/configuration/route/rule/), [DNS Route Rule](/configuration/dns/rule/)
and [Headless Rule](/configuration/rule-set/headless-rule/).

#### 1.11.0-alpha.9

* Improve tun compatibility **1**
* Fixes and improvements

**1**:

When `gvisor` tun stack is enabled, even if the request passes routing,
if the outbound connection establishment fails,
the connection still does not need to be established and a TCP RST is replied.

#### 1.11.0-alpha.7

* Introducing rule actions **1**

**1**:

New rule actions replace legacy inbound fields and special outbound fields,
and can be used for pre-matching **2**.

See [Rule](/configuration/route/rule/),
[Rule Action](/configuration/route/rule_action/),
[DNS Rule](/configuration/dns/rule/) and
[DNS Rule Action](/configuration/dns/rule_action/).

For migration, see
[Migrate legacy special outbounds to rule actions](/migration/#migrate-legacy-special-outbounds-to-rule-actions),
[Migrate legacy inbound fields to rule actions](/migration/#migrate-legacy-inbound-fields-to-rule-actions)
and [Migrate legacy DNS route options to rule actions](/migration/#migrate-legacy-dns-route-options-to-rule-actions).

**2**:

Similar to Surge's pre-matching.

Specifically, new rule actions allow you to reject connections with
TCP RST (for TCP connections) and ICMP port unreachable (for UDP packets)
before connection established to improve tun's compatibility.

See [Rule Action](/configuration/route/rule_action/).

#### 1.11.0-alpha.6

* Update quic-go to v0.48.1
* Set gateway for tun correctly
* Fixes and improvements

#### 1.11.0-alpha.2

* Add warnings for usage of deprecated features
* Fixes and improvements

#### 1.11.0-alpha.1

* Update quic-go to v0.48.0
* Fixes and improvements

### 1.10.1

* Fixes and improvements

### 1.10.0

Important changes since 1.9:

* Introducing auto-redirect **1**
* Add AdGuard DNS Filter support **2**
* TUN address fields are merged **3**
* Add custom options for `auto-route` and `auto-redirect` **4**
* Drop support for go1.18 and go1.19 **5**
* Add tailing comma support in JSON configuration
* Improve sniffers **6**
* Add new `inline` rule-set type **7**
* Add access control options for Clash API **8**
* Add `rule_set_ip_cidr_accept_empty` DNS address filter rule item **9**
* Add auto reload support for local rule-set
* Update fsnotify usages **10**
* Add IP address support for `rule-set match` command
* Add `rule-set decompile` command
* Add `process_path_regex` rule item
* Update uTLS to v1.6.7 **11**
* Optimize memory usages of rule-sets **12**

**1**:

The new auto-redirect feature allows TUN to automatically
configure connection redirection to improve proxy performance.

When auto-redirect is enabled, new route address set options will allow you to
automatically configure destination IP CIDR rules from a specified rule set to the firewall.

Specified or unspecified destinations will bypass the sing-box routes to get better performance
(for example, keep hardware offloading of direct traffics on the router).

See [TUN](/configuration/inbound/tun).

**2**:

The new feature allows you to use AdGuard DNS Filter lists in a sing-box without AdGuard Home.

See [AdGuard DNS Filter](/configuration/rule-set/adguard/).

**3**:

See [Migration](/migration/#tun-address-fields-are-merged).

**4**:

See [iproute2_table_index](/configuration/inbound/tun/#iproute2_table_index),
[iproute2_rule_index](/configuration/inbound/tun/#iproute2_rule_index),
[auto_redirect_input_mark](/configuration/inbound/tun/#auto_redirect_input_mark) and
[auto_redirect_output_mark](/configuration/inbound/tun/#auto_redirect_output_mark).

**5**:

Due to maintenance difficulties, sing-box 1.10.0 requires at least Go 1.20 to compile.

**6**:

BitTorrent, DTLS, RDP, SSH sniffers are added.

Now the QUIC sniffer can correctly extract the server name from Chromium requests and
can identify common QUIC clients, including
Chromium, Safari, Firefox, quic-go (including uquic disguised as Chrome).

**7**:

The new [rule-set](/configuration/rule-set/) type inline (which also becomes the default type)
allows you to write headless rules directly without creating a rule-set file.

**8**:

With new access control options, not only can you allow Clash dashboards
to access the Clash API on your local network,
you can also manually limit the websites that can access the API instead of allowing everyone.

See [Clash API](/configuration/experimental/clash-api/).

**9**:

See [DNS Rule](/configuration/dns/rule/#rule_set_ip_cidr_accept_empty).

**10**:

sing-box now uses fsnotify correctly and will not cancel watching
if the target file is deleted or recreated via rename (e.g. `mv`).

This affects all path options that support reload, including
`tls.certificate_path`, `tls.key_path`, `tls.ech.key_path` and `rule_set.path`.

**11**:

Some legacy chrome fingerprints have been removed and will fallback to chrome,
see [utls](/configuration/shared/tls#utls).

**12**:

See [Source Format](/configuration/rule-set/source-format/#version).

### 1.9.7

* Fixes and improvements

#### 1.10.0-beta.11

* Update uTLS to v1.6.7 **1**

**1**:

Some legacy chrome fingerprints have been removed and will fallback to chrome,
see [utls](/configuration/shared/tls#utls).

#### 1.10.0-beta.10

* Add `process_path_regex` rule item
* Fixes and improvements

_The macOS standalone versions of sing-box (>=1.9.5/<1.10.0-beta.11) now silently fail and require manual granting of
the **Full Disk Access** permission to system extension to start, probably due to Apple's changed security policy. We
will prompt users about this in feature versions._

### 1.9.6

* Fixes and improvements

### 1.9.5

* Update quic-go to v0.47.0
* Fix direct dialer not resolving domain
* Fix no error return when empty DNS cache retrieved
* Fix build with go1.23
* Fix stream sniffer
* Fix bad redirect in clash-api
* Fix wireguard events chan leak
* Fix cached conn eats up read deadlines
* Fix disconnected interface selected as default in windows
* Update Bundle Identifiers for Apple platform clients **1**

**1**:

See [Migration](/migration/#bundle-identifier-updates-in-apple-platform-clients).

We are still working on getting all sing-box apps back on the App Store, which should be completed within a week
(SFI on the App Store and others on TestFlight are already available).

#### 1.10.0-beta.8

* Fixes and improvements

_With the help of a netizen, we are in the process of getting sing-box apps back on the App Store, which should be
completed within a month (TestFlight is already available)._

#### 1.10.0-beta.7

* Update quic-go to v0.47.0
* Fixes and improvements

#### 1.10.0-beta.6

* Add RDP sniffer
* Fixes and improvements

#### 1.10.0-beta.5

* Add PNA support for [Clash API](/configuration/experimental/clash-api/)
* Fixes and improvements

#### 1.10.0-beta.3

* Add SSH sniffer
* Fixes and improvements

#### 1.10.0-beta.2

* Build with go1.23
* Fixes and improvements

### 1.9.4

* Update quic-go to v0.46.0
* Update Hysteria2 BBR congestion control
* Filter HTTPS ipv4hint/ipv6hint with domain strategy
* Fix crash on Android when using process rules
* Fix non-IP queries accepted by address filter rules
* Fix UDP server for shadowsocks AEAD multi-user inbounds
* Fix default next protos for v2ray QUIC transport
* Fix default end value of port range configuration options
* Fix reset v2ray transports
* Fix panic caused by rule-set generation of duplicate keys for `domain_suffix`
* Fix UDP connnection leak when sniffing
* Fixes and improvements

_Due to problems with our Apple developer account,
sing-box apps on Apple platforms are temporarily unavailable for download or update.
If your company or organization is willing to help us return to the App Store,
please [contact us](mailto:contact@sagernet.org)._

#### 1.10.0-alpha.29

* Update quic-go to v0.46.0
* Fixes and improvements

#### 1.10.0-alpha.25

* Add AdGuard DNS Filter support **1**

**1**:

The new feature allows you to use AdGuard DNS Filter lists in a sing-box without AdGuard Home.

See [AdGuard DNS Filter](/configuration/rule-set/adguard/).

#### 1.10.0-alpha.23

* Add Chromium support for QUIC sniffer
* Add client type detect support for QUIC sniffer **1**
* Fixes and improvements

**1**:

Now the QUIC sniffer can correctly extract the server name from Chromium requests and
can identify common QUIC clients, including
Chromium, Safari, Firefox, quic-go (including uquic disguised as Chrome).

See [Protocol Sniff](/configuration/route/sniff/) and [Route Rule](/configuration/route/rule/#client).

#### 1.10.0-alpha.22

* Optimize memory usages of rule-sets **1**
* Fixes and improvements

**1**:

See [Source Format](/configuration/rule-set/source-format/#version).

#### 1.10.0-alpha.20

* Add DTLS sniffer
* Fixes and improvements

#### 1.10.0-alpha.19

* Add `rule-set decompile` command
* Add IP address support for `rule-set match` command
* Fixes and improvements

#### 1.10.0-alpha.18

* Add new `inline` rule-set type **1**
* Add auto reload support for local rule-set
* Update fsnotify usages **2**
* Fixes and improvements

**1**:

The new [rule-set](/configuration/rule-set/) type inline (which also becomes the default type)
allows you to write headless rules directly without creating a rule-set file.

**2**:

sing-box now uses fsnotify correctly and will not cancel watching
if the target file is deleted or recreated via rename (e.g. `mv`).

This affects all path options that support reload, including
`tls.certificate_path`, `tls.key_path`, `tls.ech.key_path` and `rule_set.path`.

#### 1.10.0-alpha.17

* Some chaotic changes **1**
* `rule_set_ipcidr_match_source` rule items are renamed **2**
* Add `rule_set_ip_cidr_accept_empty` DNS address filter rule item **3**
* Update quic-go to v0.45.1
* Fixes and improvements

**1**:

Something may be broken, please actively report problems with this version.

**2**:

`rule_set_ipcidr_match_source` route and DNS rule items are renamed to
`rule_set_ip_cidr_match_source` and will be remove in sing-box 1.11.0.

**3**:

See [DNS Rule](/configuration/dns/rule/#rule_set_ip_cidr_accept_empty).

#### 1.10.0-alpha.16

* Add custom options for `auto-route` and `auto-redirect` **1**
* Fixes and improvements

**1**:

See [iproute2_table_index](/configuration/inbound/tun/#iproute2_table_index),
[iproute2_rule_index](/configuration/inbound/tun/#iproute2_rule_index),
[auto_redirect_input_mark](/configuration/inbound/tun/#auto_redirect_input_mark) and
[auto_redirect_output_mark](/configuration/inbound/tun/#auto_redirect_output_mark).

#### 1.10.0-alpha.13

* TUN address fields are merged **1**
* Add route address set support for auto-redirect **2**

**1**:

See [Migration](/migration/#tun-address-fields-are-merged).

**2**:

The new feature will allow you to configure the destination IP CIDR rules
in the specified rule-sets to the firewall automatically.

Specified or unspecified destinations will bypass the sing-box routes to get better performance
(for example, keep hardware offloading of direct traffics on the router).

See [route_address_set](/configuration/inbound/tun/#route_address_set)
and [route_exclude_address_set](/configuration/inbound/tun/#route_exclude_address_set).

#### 1.10.0-alpha.12

* Fix auto-redirect not configuring nftables forward chain correctly
* Fixes and improvements

### 1.9.3

* Fixes and improvements

#### 1.10.0-alpha.10

* Fixes and improvements

### 1.9.2

* Fixes and improvements

#### 1.10.0-alpha.8

* Drop support for go1.18 and go1.19 **1**
* Update quic-go to v0.45.0
* Update Hysteria2 BBR congestion control
* Fixes and improvements

**1**:

Due to maintenance difficulties, sing-box 1.10.0 requires at least Go 1.20 to compile.

### 1.9.1

* Fixes and improvements

#### 1.10.0-alpha.7

* Fixes and improvements

#### 1.10.0-alpha.5

* Improve auto-redirect **1**

**1**:

nftables support and DNS hijacking has been added.

Tun inbounds with `auto_route` and `auto_redirect` now works as expected on routers **without intervention**.

#### 1.10.0-alpha.4

* Fix auto-redirect **1**
* Improve auto-route on linux **2**

**1**:

Tun inbounds with `auto_route` and `auto_redirect` now works as expected on routers.

**2**:

Tun inbounds with `auto_route` and `strict_route` now works as expected on routers and servers,
but the usages of [exclude_interface](/configuration/inbound/tun/#exclude_interface) need to be updated.

#### 1.10.0-alpha.2

* Move auto-redirect to Tun **1**
* Fixes and improvements

**1**:

Linux support are added.

See [Tun](/configuration/inbound/tun/#auto_redirect).

#### 1.10.0-alpha.1

* Add tailing comma support in JSON configuration
* Add simple auto-redirect for Android **1**
* Add BitTorrent sniffer **2**

**1**:

It allows you to use redirect inbound in the sing-box Android client
and automatically configures IPv4 TCP redirection via su.

This may alleviate the symptoms of some OCD patients who think that
redirect can effectively save power compared to the system HTTP Proxy.

See [Redirect](/configuration/inbound/redirect/).

**2**:

See [Protocol Sniff](/configuration/route/sniff/).

### 1.9.0

* Fixes and improvements

Important changes since 1.8:

* `domain_suffix` behavior update **1**
* `process_path` format update on Windows **2**
* Add address filter DNS rule items **3**
* Add support for `client-subnet` DNS options **4**
* Add rejected DNS response cache support **5**
* Add `bypass_domain` and `search_domain` platform HTTP proxy options **6**
* Fix missing `rule_set_ipcidr_match_source` item in DNS rules **7**
* Handle Windows power events
* Always disable cache for fake-ip DNS transport if `dns.independent_cache` disabled
* Improve DNS truncate behavior
* Update Hysteria protocol
* Update quic-go to v0.43.1
* Update gVisor to 20240422.0
* Mitigating TunnelVision attacks **8**

**1**:

See [Migration](/migration/#domain_suffix-behavior-update).

**2**:

See [Migration](/migration/#process_path-format-update-on-windows).

**3**:

The new DNS feature allows you to more precisely bypass Chinese websites via **DNS leaks**. Do not use plain local DNS
if using this method.

See [Address Filter Fields](/configuration/dns/rule#address-filter-fields).

[Client example](/manual/proxy/client#traffic-bypass-usage-for-chinese-users) updated.

**4**:

See [DNS](/configuration/dns), [DNS Server](/configuration/dns/server) and [DNS Rules](/configuration/dns/rule).

Since this feature makes the scenario mentioned in `alpha.1` no longer leak DNS requests,
the [Client example](/manual/proxy/client#traffic-bypass-usage-for-chinese-users) has been updated.

**5**:

The new feature allows you to cache the check results of
[Address filter DNS rule items](/configuration/dns/rule/#address-filter-fields) until expiration.

**6**:

See [TUN](/configuration/inbound/tun) inbound.

**7**:

See [DNS Rule](/configuration/dns/rule/).

**8**:

See [TunnelVision](/manual/misc/tunnelvision).

#### 1.9.0-rc.22

* Fixes and improvements

#### 1.9.0-rc.20

* Prioritize `*_route_address` in linux auto-route
* Fix `*_route_address` in darwin auto-route

#### 1.8.14

* Fix hysteria2 panic
* Fixes and improvements

#### 1.9.0-rc.18

* Add custom prefix support in EDNS0 client subnet options
* Fix hysteria2 crash
* Fix `store_rdrc` corrupted
* Update quic-go to v0.43.1
* Fixes and improvements

#### 1.9.0-rc.16

* Mitigating TunnelVision attacks **1**
* Fixes and improvements

**1**:

See [TunnelVision](/manual/misc/tunnelvision).

#### 1.9.0-rc.15

* Fixes and improvements

#### 1.8.13

* Fix fake-ip mapping
* Fixes and improvements

#### 1.9.0-rc.14

* Fixes and improvements

#### 1.9.0-rc.13

* Update Hysteria protocol
* Update quic-go to v0.43.0
* Update gVisor to 20240422.0
* Fixes and improvements

#### 1.8.12

* Now we have official APT and DNF repositories **1**
* Fix packet MTU for QUIC protocols
* Fixes and improvements

**1**:

Including stable and beta versions, see https://sing-box.sagernet.org/installation/package-manager/

#### 1.9.0-rc.11

* Fixes and improvements

#### 1.8.11

* Fixes and improvements

#### 1.8.10

* Fixes and improvements

#### 1.9.0-beta.17

* Update `quic-go` to v0.42.0
* Fixes and improvements

#### 1.9.0-beta.16

* Fixes and improvements

_Our Testflight distribution has been temporarily blocked by Apple (possibly due to too many beta versions)
and you cannot join the test, install or update the sing-box beta app right now.
Please wait patiently for processing._

#### 1.9.0-beta.14

* Update gVisor to 20240212.0-65-g71212d503
* Fixes and improvements

#### 1.8.9

* Fixes and improvements

#### 1.8.8

* Fixes and improvements

#### 1.9.0-beta.7

* Fixes and improvements

#### 1.9.0-beta.6

* Fix address filter DNS rule items **1**
* Fix DNS outbound responding with wrong data
* Fixes and improvements

**1**:

Fixed an issue where address filter DNS rule was incorrectly rejected under certain circumstances.
If you have enabled `store_rdrc` to save results, consider clearing the cache file.

#### 1.8.7

* Fixes and improvements

#### 1.9.0-alpha.15

* Fixes and improvements

#### 1.9.0-alpha.14

* Improve DNS truncate behavior
* Fixes and improvements

#### 1.9.0-alpha.13

* Fixes and improvements

#### 1.8.6

* Fixes and improvements

#### 1.9.0-alpha.12

* Handle Windows power events
* Always disable cache for fake-ip DNS transport if `dns.independent_cache` disabled
* Fixes and improvements

#### 1.9.0-alpha.11

* Fix missing `rule_set_ipcidr_match_source` item in DNS rules **1**
* Fixes and improvements

**1**:

See [DNS Rule](/configuration/dns/rule/).

#### 1.9.0-alpha.10

* Add `bypass_domain` and `search_domain` platform HTTP proxy options **1**
* Fixes and improvements

**1**:

See [TUN](/configuration/inbound/tun) inbound.

#### 1.9.0-alpha.8

* Add rejected DNS response cache support **1**
* Fixes and improvements

**1**:

The new feature allows you to cache the check results of
[Address filter DNS rule items](/configuration/dns/rule/#address-filter-fields) until expiration.

#### 1.9.0-alpha.7

* Update gVisor to 20240206.0
* Fixes and improvements

#### 1.9.0-alpha.6

* Fixes and improvements

#### 1.9.0-alpha.3

* Update `quic-go` to v0.41.0
* Fixes and improvements

#### 1.9.0-alpha.2

* Add support for `client-subnet` DNS options **1**
* Fixes and improvements

**1**:

See [DNS](/configuration/dns), [DNS Server](/configuration/dns/server) and [DNS Rules](/configuration/dns/rule).

Since this feature makes the scenario mentioned in `alpha.1` no longer leak DNS requests,
the [Client example](/manual/proxy/client#traffic-bypass-usage-for-chinese-users) has been updated.

#### 1.9.0-alpha.1

* `domain_suffix` behavior update **1**
* `process_path` format update on Windows **2**
* Add address filter DNS rule items **3**

**1**:

See [Migration](/migration/#domain_suffix-behavior-update).

**2**:

See [Migration](/migration/#process_path-format-update-on-windows).

**3**:

The new DNS feature allows you to more precisely bypass Chinese websites via **DNS leaks**. Do not use plain local DNS
if using this method.

See [Address Filter Fields](/configuration/dns/rule#address-filter-fields).

[Client example](/manual/proxy/client#traffic-bypass-usage-for-chinese-users) updated.

#### 1.8.5

* Fixes and improvements

#### 1.8.4

* Fixes and improvements

#### 1.8.2

* Fixes and improvements

#### 1.8.1

* Fixes and improvements

### 1.8.0

* Fixes and improvements

Important changes since 1.7:

* Migrate cache file from Clash API to independent options **1**
* Introducing [rule-set](/configuration/rule-set/) **2**
* Add `sing-box geoip`, `sing-box geosite` and `sing-box rule-set` commands **3**
* Allow nested logical rules **4**
* Independent `source_ip_is_private` and `ip_is_private` rules **5**
* Add context to JSON decode error message **6**
* Reject internal fake-ip queries **7**
* Add GSO support for TUN and WireGuard system interface **8**
* Add `idle_timeout` for URLTest outbound **9**
* Add simple loopback detect
* Optimize memory usage of idle connections
* Update uTLS to 1.5.4 **10**
* Update dependencies **11**

**1**:

See [Cache File](/configuration/experimental/cache-file/) and
[Migration](/migration/#migrate-cache-file-from-clash-api-to-independent-options).

**2**:

rule-set is independent collections of rules that can be compiled into binaries to improve performance.
Compared to legacy GeoIP and Geosite resources,
it can include more types of rules, load faster,
use less memory, and update automatically.

See [Route#rule_set](/configuration/route/#rule_set),
[Route Rule](/configuration/route/rule/),
[DNS Rule](/configuration/dns/rule/),
[rule-set](/configuration/rule-set/),
[Source Format](/configuration/rule-set/source-format/) and
[Headless Rule](/configuration/rule-set/headless-rule/).

For GEO resources migration, see [Migrate GeoIP to rule-sets](/migration/#migrate-geoip-to-rule-sets) and
[Migrate Geosite to rule-sets](/migration/#migrate-geosite-to-rule-sets).

**3**:

New commands manage GeoIP, Geosite and rule-set resources, and help you migrate GEO resources to rule-sets.

**4**:

Logical rules in route rules, DNS rules, and the new headless rule now allow nesting of logical rules.

**5**:

The `private` GeoIP country never existed and was actually implemented inside V2Ray.
Since GeoIP was deprecated, we made this rule independent, see [Migration](/migration/#migrate-geoip-to-rule-sets).

**6**:

JSON parse errors will now include the current key path.
Only takes effect when compiled with Go 1.21+.

**7**:

All internal DNS queries now skip DNS rules with `server` type `fakeip`,
and the default DNS server can no longer be `fakeip`.

This change is intended to break incorrect usage and essentially requires no action.

**8**:

See [TUN](/configuration/inbound/tun/) inbound and [WireGuard](/configuration/outbound/wireguard/) outbound.

**9**:

When URLTest is idle for a certain period of time, the scheduled delay test will be paused.

**10**:

Added some new [fingerprints](/configuration/shared/tls#utls).
Also, starting with this release, uTLS requires at least Go 1.20.

**11**:

Updated `cloudflare-tls`, `gomobile`, `smux`, `tfo-go` and `wireguard-go` to latest, `quic-go` to `0.40.1` and  `gvisor`
to `20231204.0`

#### 1.8.0-rc.11

* Fixes and improvements

#### 1.7.8

* Fixes and improvements

#### 1.8.0-rc.10

* Fixes and improvements

#### 1.7.7

* Fix V2Ray transport `path` validation behavior **1**
* Fixes and improvements

**1**:

See [V2Ray transport](/configuration/shared/v2ray-transport/).

#### 1.8.0-rc.7

* Fixes and improvements

#### 1.8.0-rc.3

* Fix V2Ray transport `path` validation behavior **1**
* Fixes and improvements

**1**:

See [V2Ray transport](/configuration/shared/v2ray-transport/).

#### 1.7.6

* Fixes and improvements

#### 1.8.0-rc.1

* Fixes and improvements

#### 1.8.0-beta.9

* Add simple loopback detect
* Fixes and improvements

#### 1.7.5

* Fixes and improvements

#### 1.8.0-alpha.17

* Add GSO support for TUN and WireGuard system interface **1**
* Update uTLS to 1.5.4 **2**
* Update dependencies **3**
* Fixes and improvements

**1**:

See [TUN](/configuration/inbound/tun/) inbound and [WireGuard](/configuration/outbound/wireguard/) outbound.

**2**:

Added some new [fingerprints](/configuration/shared/tls#utls).
Also, starting with this release, uTLS requires at least Go 1.20.

**3**:

Updated `cloudflare-tls`, `gomobile`, `smux`, `tfo-go` and `wireguard-go` to latest, and `gvisor` to `20231204.0`

This may break something, good luck!

#### 1.7.4

* Fixes and improvements

_Due to the long waiting time, this version is no longer waiting for approval
by the Apple App Store, so updates to Apple Platforms will be delayed._

#### 1.8.0-alpha.16

* Fixes and improvements

#### 1.8.0-alpha.15

* Some chaotic changes **1**
* Fixes and improvements

**1**:

Designed to optimize memory usage of idle connections, may take effect on the following protocols:

| Protocol                                             | TCP              | UDP              |
|------------------------------------------------------|------------------|------------------|
| HTTP proxy server                                    | :material-check: | /                |
| SOCKS5                                               | :material-close: | :material-check: |
| Shadowsocks none/AEAD/AEAD2022                       | :material-check: | :material-check: |
| Trojan                                               | /                | :material-check: |
| TUIC/Hysteria/Hysteria2                              | :material-close: | :material-check: |
| Multiplex                                            | :material-close: | :material-check: |
| Plain TLS (Trojan/VLESS without extra sub-protocols) | :material-check: | /                |
| Other protocols                                      | :material-close: | :material-close: |

At the same time, everything existing may be broken, please actively report problems with this version.

#### 1.8.0-alpha.13

* Fixes and improvements

#### 1.8.0-alpha.10

* Add `idle_timeout` for URLTest outbound **1**
* Fixes and improvements

**1**:

When URLTest is idle for a certain period of time, the scheduled delay test will be paused.

#### 1.7.2

* Fixes and improvements

#### 1.8.0-alpha.8

* Add context to JSON decode error message **1**
* Reject internal fake-ip queries **2**
* Fixes and improvements

**1**:

JSON parse errors will now include the current key path.
Only takes effect when compiled with Go 1.21+.

**2**:

All internal DNS queries now skip DNS rules with `server` type `fakeip`,
and the default DNS server can no longer be `fakeip`.

This change is intended to break incorrect usage and essentially requires no action.

#### 1.8.0-alpha.7

* Fixes and improvements

#### 1.7.1

* Fixes and improvements

#### 1.8.0-alpha.6

* Fix rule-set matching logic **1**
* Fixes and improvements

**1**:

Now the rules in the `rule_set` rule item can be logically considered to be merged into the rule using rule-sets,
rather than completely following the AND logic.

#### 1.8.0-alpha.5

* Parallel rule-set initialization
* Independent `source_ip_is_private` and `ip_is_private` rules **1**

**1**:

The `private` GeoIP country never existed and was actually implemented inside V2Ray.
Since GeoIP was deprecated, we made this rule independent, see [Migration](/migration/#migrate-geoip-to-rule-sets).

#### 1.8.0-alpha.1

* Migrate cache file from Clash API to independent options **1**
* Introducing [rule-set](/configuration/rule-set/) **2**
* Add `sing-box geoip`, `sing-box geosite` and `sing-box rule-set` commands **3**
* Allow nested logical rules **4**

**1**:

See [Cache File](/configuration/experimental/cache-file/) and
[Migration](/migration/#migrate-cache-file-from-clash-api-to-independent-options).

**2**:

rule-set is independent collections of rules that can be compiled into binaries to improve performance.
Compared to legacy GeoIP and Geosite resources,
it can include more types of rules, load faster,
use less memory, and update automatically.

See [Route#rule_set](/configuration/route/#rule_set),
[Route Rule](/configuration/route/rule/),
[DNS Rule](/configuration/dns/rule/),
[rule-set](/configuration/rule-set/),
[Source Format](/configuration/rule-set/source-format/) and
[Headless Rule](/configuration/rule-set/headless-rule/).

For GEO resources migration, see [Migrate GeoIP to rule-sets](/migration/#migrate-geoip-to-rule-sets) and
[Migrate Geosite to rule-sets](/migration/#migrate-geosite-to-rule-sets).

**3**:

New commands manage GeoIP, Geosite and rule-set resources, and help you migrate GEO resources to rule-sets.

**4**:

Logical rules in route rules, DNS rules, and the new headless rule now allow nesting of logical rules.

### 1.7.0

* Fixes and improvements

Important changes since 1.6:

* Add [exclude route support](/configuration/inbound/tun/) for TUN inbound
* Add `udp_disable_domain_unmapping` [inbound listen option](/configuration/shared/listen/) **1**
* Add [HTTPUpgrade V2Ray transport](/configuration/shared/v2ray-transport#HTTPUpgrade) support **2**
* Migrate multiplex and UoT server to inbound **3**
* Add TCP Brutal support for multiplex **4**
* Add `wifi_ssid` and `wifi_bssid` route and DNS rules **5**
* Update quic-go to v0.40.0
* Update gVisor to 20231113.0

**1**:

If enabled, for UDP proxy requests addressed to a domain,
the original packet address will be sent in the response instead of the mapped domain.

This option is used for compatibility with clients that
do not support receiving UDP packets with domain addresses, such as Surge.

**2**:

Introduced in V2Ray 5.10.0.

The new HTTPUpgrade transport has better performance than WebSocket and is better suited for CDN abuse.

**3**:

Starting in 1.7.0, multiplexing support is no longer enabled by default
and needs to be turned on explicitly in inbound
options.

**4**

Hysteria Brutal Congestion Control Algorithm in TCP. A kernel module needs to be installed on the Linux server,
see [TCP Brutal](/configuration/shared/tcp-brutal/) for details.

**5**:

Only supported in graphical clients on Android and Apple platforms.

#### 1.7.0-rc.3

* Fixes and improvements

#### 1.6.7

* macOS: Add button for uninstall SystemExtension in the standalone graphical client
* Fix missing UDP user context on TUIC/Hysteria2 inbounds
* Fixes and improvements

#### 1.7.0-rc.2

* Fix missing UDP user context on TUIC/Hysteria2 inbounds
* macOS: Add button for uninstall SystemExtension in the standalone graphical client

#### 1.6.6

* Fixes and improvements

#### 1.7.0-rc.1

* Fixes and improvements

#### 1.7.0-beta.5

* Update gVisor to 20231113.0
* Fixes and improvements

#### 1.7.0-beta.4

* Add `wifi_ssid` and `wifi_bssid` route and DNS rules **1**
* Fixes and improvements

**1**:

Only supported in graphical clients on Android and Apple platforms.

#### 1.7.0-beta.3

* Fix zero TTL was incorrectly reset
* Fixes and improvements

#### 1.6.5

* Fix crash if TUIC inbound authentication failed
* Fixes and improvements

#### 1.7.0-beta.2

* Fix crash if TUIC inbound authentication failed
* Update quic-go to v0.40.0
* Fixes and improvements

#### 1.6.4

* Fixes and improvements

#### 1.7.0-beta.1

* Fixes and improvements

#### 1.6.3

* iOS/Android: Fix profile auto update
* Fixes and improvements

#### 1.7.0-alpha.11

* iOS/Android: Fix profile auto update
* Fixes and improvements

#### 1.7.0-alpha.10

* Fix tcp-brutal not working with TLS
* Fix Android client not closing in some cases
* Fixes and improvements

#### 1.6.2

* Fixes and improvements

#### 1.6.1

* Our [Android client](/installation/clients/sfa/) is now available in the Google Play Store ▶️
* Fixes and improvements

#### 1.7.0-alpha.6

* Fixes and improvements

#### 1.7.0-alpha.4

* Migrate multiplex and UoT server to inbound **1**
* Add TCP Brutal support for multiplex **2**

**1**:

Starting in 1.7.0, multiplexing support is no longer enabled by default and needs to be turned on explicitly in inbound
options.

**2**

Hysteria Brutal Congestion Control Algorithm in TCP. A kernel module needs to be installed on the Linux server,
see [TCP Brutal](/configuration/shared/tcp-brutal/) for details.

#### 1.7.0-alpha.3

* Add [HTTPUpgrade V2Ray transport](/configuration/shared/v2ray-transport#HTTPUpgrade) support **1**
* Fixes and improvements

**1**:

Introduced in V2Ray 5.10.0.

The new HTTPUpgrade transport has better performance than WebSocket and is better suited for CDN abuse.

### 1.6.0

* Fixes and improvements

Important changes since 1.5:

* Our [Apple tvOS client](/installation/clients/sft/) is now available in the App Store 🍎
* Update BBR congestion control for TUIC and Hysteria2 **1**
* Update brutal congestion control for Hysteria2
* Add `brutal_debug` option for Hysteria2
* Update legacy Hysteria protocol **2**
* Add TLS self sign key pair generate command
* Remove [Deprecated Features](/deprecated/) by agreement

**1**:

None of the existing Golang BBR congestion control implementations have been reviewed or unit tested.
This update is intended to address the multi-send defects of the old implementation and may introduce new issues.

**2**

Based on discussions with the original author, the brutal CC and QUIC protocol parameters of
the old protocol (Hysteria 1) have been updated to be consistent with Hysteria 2

#### 1.7.0-alpha.2

* Fix bugs introduced in 1.7.0-alpha.1

#### 1.7.0-alpha.1

* Add [exclude route support](/configuration/inbound/tun/) for TUN inbound
* Add `udp_disable_domain_unmapping` [inbound listen option](/configuration/shared/listen/) **1**
* Fixes and improvements

**1**:

If enabled, for UDP proxy requests addressed to a domain,
the original packet address will be sent in the response instead of the mapped domain.

This option is used for compatibility with clients that
do not support receiving UDP packets with domain addresses, such as Surge.

#### 1.5.5

* Fix IPv6 `auto_route` for Linux **1**
* Add legacy builds for old Windows and macOS systems **2**
* Fixes and improvements

**1**:

When `auto_route` is enabled and `strict_route` is disabled, the device can now be reached from external IPv6 addresses.

**2**:

Built using Go 1.20, the last version that will run on
Windows 7, 8, Server 2008, Server 2012 and macOS 10.13 High
Sierra, 10.14 Mojave.

#### 1.6.0-rc.4

* Fixes and improvements

#### 1.6.0-rc.1

* Add legacy builds for old Windows and macOS systems **1**
* Fixes and improvements

**1**:

Built using Go 1.20, the last version that will run on
Windows 7, 8, Server 2008, Server 2012 and macOS 10.13 High
Sierra, 10.14 Mojave.

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

* Our [Apple tvOS client](/installation/clients/sft/) is now available in the App Store 🍎
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
* Remove [Deprecated Features](/deprecated/) by agreement
* Fixes and improvements

**1**:

None of the existing Golang BBR congestion control implementations have been reviewed or unit tested.
This update is intended to address the multi-send defects of the old implementation and may introduce new issues.

### 1.5.0

* Fixes and improvements

Important changes since 1.4:

* Add TLS [ECH server](/configuration/shared/tls/) support
* Improve TLS TCH client configuration
* Add TLS ECH key pair generator **1**
* Add TLS ECH support for QUIC based protocols **2**
* Add KDE support for the `set_system_proxy` option in HTTP inbound
* Add Hysteria2 protocol support **3**
* Add `interrupt_exist_connections` option for `Selector` and `URLTest` outbounds **4**
* Add DNS01 challenge support for ACME TLS certificate issuer **5**
* Add `merge` command **6**
* Mark [Deprecated Features](/deprecated/)

**1**:

Command: `sing-box generate ech-keypair <plain_server_name> [--pq-signature-schemes-enabled]`

**2**:

All inbounds and outbounds are supported, including `Naiveproxy`, `Hysteria[/2]`, `TUIC` and `V2ray QUIC transport`.

**3**:

See [Hysteria2 inbound](/configuration/inbound/hysteria2/) and [Hysteria2 outbound](/configuration/outbound/hysteria2/)

For protocol description, please refer to [https://v2.hysteria.network](https://v2.hysteria.network)

**4**:

Interrupt existing connections when the selected outbound has changed.

Only inbound connections are affected by this setting, internal connections will always be interrupted.

**5**:

Only `Alibaba Cloud DNS` and `Cloudflare` are supported, see [ACME Fields](/configuration/shared/tls#acme-fields)
and [DNS01 Challenge Fields](/configuration/shared/dns01_challenge/).

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
and [DNS01 Challenge Fields](/configuration/shared/dns01_challenge/).

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
* Mark [deprecated features](/deprecated/)

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

See [Hysteria2 inbound](/configuration/inbound/hysteria2/) and [Hysteria2 outbound](/configuration/outbound/hysteria2/)

For protocol description, please refer to [https://v2.hysteria.network](https://v2.hysteria.network)

#### 1.5.0-beta.1

* Add TLS [ECH server](/configuration/shared/tls/) support
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

### 1.4.0

* Fix bugs and update dependencies

Important changes since 1.3:

* Add TUIC support **1**
* Add `udp_over_stream` option for TUIC client **2**
* Add MultiPath TCP support **3**
* Add `include_interface` and `exclude_interface` options for tun inbound
* Pause recurring tasks when no network or device idle
* Improve Android and Apple platform clients

*1*:

See [TUIC inbound](/configuration/inbound/tuic/)
and [TUIC outbound](/configuration/outbound/tuic/)

**2**:

This is the TUIC port of the [UDP over TCP protocol](/configuration/shared/udp-over-tcp/), designed to provide a QUIC
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

This is the TUIC port of the [UDP over TCP protocol](/configuration/shared/udp-over-tcp/), designed to provide a QUIC
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

See [TUIC inbound](/configuration/inbound/tuic/)
and [TUIC outbound](/configuration/outbound/tuic/)

#### 1.3.6

* Fixes and improvements

#### 1.3.5

* Fixes and improvements
* Introducing our [Apple tvOS](/installation/clients/sft/) client applications **1**
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

* Introducing our [new iOS](/installation/clients/sfi/) and [macOS](/installation/clients/sfm/) client applications **1
  **
* Fixes and improvements

**1**:

The old testflight link and app are no longer valid.

#### 1.3.1-beta.2

* Fix bugs and update dependencies

#### 1.3.1-beta.1

* Fixes and improvements

### 1.3.0

* Fix bugs and update dependencies

Important changes since 1.2:

* Add [FakeIP](/configuration/dns/fakeip/) support **1**
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

See [FAQ](/faq/fakeip/) for more information.

*2*:

Added new `h2mux` multiplex protocol and `padding` multiplex option, see [Multiplex](/configuration/shared/multiplex/).

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

Added new `h2mux` multiplex protocol and `padding` multiplex option, see [Multiplex](/configuration/shared/multiplex/).

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
* Add [L3 routing](/configuration/route/ip-rule/) support **1**
* Add `rewrite_ttl` DNS rule action
* Add [FakeIP](/configuration/dns/fakeip/) support **2**
* Add `store_fakeip` Clash API option
* Add multi-peer support for [WireGuard](/configuration/outbound/wireguard#peers) outbound
* Add loopback detect

*1*:

It can currently be used to [route connections directly to WireGuard](/examples/wireguard-direct/) or block connections
at the IP layer.

*2*:

See [FAQ](/faq/fakeip/) for more information.

#### 1.2.3

* Introducing our [new Android client application](/installation/clients/sfa/)
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

### 1.2.0

* Fix bugs and update dependencies

Important changes since 1.1:

* Introducing our [new iOS client application](/installation/clients/sfi/)
* Introducing [UDP over TCP protocol version 2](/configuration/shared/udp-over-tcp/)
* Add [platform options](/configuration/inbound/tun#platform) for tun inbound
* Add [ShadowTLS protocol v3](https://github.com/ihciah/shadow-tls/blob/master/docs/protocol-v3-en.md)
* Add [VLESS server](/configuration/inbound/vless/) and [vision](/configuration/outbound/vless#flow) support
* Add [reality TLS](/configuration/shared/tls/) support
* Add [NTP service](/configuration/ntp/)
* Add [DHCP DNS server](/configuration/dns/server/) support
* Add SSH [host key validation](/configuration/outbound/ssh/) support
* Add [query_type](/configuration/dns/rule/) DNS rule item
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

* Introducing the [UDP over TCP protocol version 2](/configuration/shared/udp-over-tcp/)
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

* Introducing our [new iOS client application](/installation/clients/sfi/)
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

* Add [VLESS server](/configuration/inbound/vless/) and [vision](/configuration/outbound/vless#flow) support
* Add [reality TLS](/configuration/shared/tls/) support
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

* Add [NTP service](/configuration/ntp/)
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

* Add [DHCP DNS server](/configuration/dns/server/) support
* Add SSH [host key validation](/configuration/outbound/ssh/) support
* Add [query_type](/configuration/dns/rule/) DNS rule item
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

* Add [URLTest outbound](/configuration/outbound/urltest/)
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
* Add [ShadowsocksR outbound](/configuration/outbound/shadowsocksr/)
* Add [VLESS outbound and XUDP](/configuration/outbound/vless/)
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
see [Experimental](/configuration/experimental/).

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
* Update documentation for [Dial Fields](/configuration/shared/dial/)

#### 1.0-beta3

* Add [chained inbound](/configuration/shared/listen#detour) support
* Add process_path rule item
* Add macOS redirect support
* Add ShadowTLS [Inbound](/configuration/inbound/shadowtls/), [Outbound](/configuration/outbound/shadowtls/)
  and [Examples](/examples/shadowtls/)
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

* Add [V2Ray Transport](/configuration/shared/v2ray-transport/) support for VMess and Trojan
* Allow plain http request in Naive inbound (It can now be used with nginx)
* Add proxy protocol support
* Free memory after start
* Parse X-Forward-For in HTTP requests
* Handle SIGHUP signal

##### 2022/08/22

* Add strategy setting for each [DNS server](/configuration/dns/server/)
* Add bind address to outbound options

##### 2022/08/21

* Add [Tor outbound](/configuration/outbound/tor/)
* Add [SSH outbound](/configuration/outbound/ssh/)

##### 2022/08/20

* Attempt to unwrap ip-in-fqdn socksaddr
* Fix read packages in android 12
* Fix route on some android devices
* Improve linux process searcher
* Fix write socks5 username password auth request
* Skip bind connection with private destination to interface
* Add [Trojan connection fallback](/configuration/inbound/trojan#fallback)

##### 2022/08/19

* Add Hysteria [Inbound](/configuration/inbound/hysteria/) and [Outbund](/configuration/outbound/hysteria/)
* Add [ACME TLS certificate issuer](/configuration/shared/tls/)
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
* Add [WireGuard](/configuration/outbound/wireguard/) outbound

##### 2022/08/15

* Add uid, android user and package rules support in [Tun](/configuration/inbound/tun/) routing.

##### 2022/08/13

* Fix dns concurrent write

##### 2022/08/12

* Performance improvements
* Add UoT option for [SOCKS](/configuration/outbound/socks/) outbound

##### 2022/08/11

* Add UoT option for [Shadowsocks](/configuration/outbound/shadowsocks/) outbound, UoT support for all inbounds

##### 2022/08/10

* Add full-featured [Naive](/configuration/inbound/naive/) inbound
* Fix default dns server option [#9] by iKirby

##### 2022/08/09

No changelog before.

[#9]: https://github.com/SagerNet/sing-box/pull/9
