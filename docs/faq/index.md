# Frequently Asked Questions (FAQ)

## Design

#### Why does sing-box not have feature X?

Every program contains novel features and omits someone's favorite feature. sing-box is designed with an eye to the
needs of performance, lightweight, usability, modularity, and code quality. Your favorite feature may be missing because
it doesn't fit, because it compromises performance or design clarity, or because it's a bad idea.

If it bothers you that sing-box is missing feature X, please forgive us and investigate the features that sing-box does
have. You might find that they compensate in interesting ways for the lack of X.

#### Naive outbound

NaïveProxy's main function is chromium's network stack, and it makes no sense to implement only its transport protocol.

#### Protocol combinations

The "underlying transport" in v2ray-core is actually a combination of a number of proprietary protocols and uses the
names of their upstream protocols, resulting in a great deal of Linguistic corruption.

For example, Trojan with v2ray's proprietary gRPC protocol, called Trojan gRPC by the v2ray community, is not actually a
protocol and has no role outside of abusing CDNs.

## Tun

#### What is tun?

tun is a virtual network device in unix systems, and in windows there is wintun developed by WireGuard as an
alternative. The tun module of sing-box includes traffic processing, automatic routing, and network device listening,
and is mainly used as a transparent proxy.

#### How is it different from system proxy?

System proxy usually only supports TCP and is not accepted by all applications, but tun can handle all traffic.

#### How is it different from traditional transparent proxy?

They are usually only supported under Linux and require manipulation of firewalls like iptables, while tun only modifies
the routing table.

The tproxy UDP is considered to have poor performance due to the need to create a new connection every write back in
v2ray and clash, but it is optimized in sing-box so you can still use it if needed.

#### How does it handle DNS?

In traditional transparent proxies, it is usually necessary to manually hijack port 53 to the DNS proxy server, while
tun is more flexible.

sing-box's `auto_route` will hijack all DNS requests except on [macOS and Android](./known-issues#dns).

You need to manually configure how to handle tun hijacked DNS traffic, see [Hijack DNS](/examples/dns-hijack).

#### Why I can't use it with other local proxies (e.g. via socks)?

Tun will hijack all traffic, including other proxy applications. In order to make tun work with other applications, you
need to create an inbound to proxy traffic from other applications or make them bypass the route.