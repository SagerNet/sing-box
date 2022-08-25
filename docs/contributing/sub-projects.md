The sing-box uses the following projects which also need to be maintained:

#### sing

Link: [GitHub repository](https://github.com/SagerNet/sing)

As a base tool library, there are no dependencies other than `golang.org/x/sys`.

#### sing-dns

Link: [GitHub repository](https://github.com/SagerNet/sing-dns)

Handles DNS lookups and caching.

#### sing-tun

Link: [GitHub repository](https://github.com/SagerNet/sing-tun)

Handle Tun traffic forwarding, configure routing, monitor network and routing.

This library needs to periodically update its dependency gVisor (according to tags), including checking for changes to
the used parts of the code and updating its usage. If you are involved in maintenance, you also need to check that if it
works or contains memory leaks.

#### sing-shadowsocks

Link: [GitHub repository](https://github.com/SagerNet/sing-shadowsocks)

Provides Shadowsocks client and server

#### sing-vmess

Link: [GitHub repository](https://github.com/SagerNet/sing-vmess)

Provides VMess client and server

#### netlink

Link: [GitHub repository](https://github.com/SagerNet/netlink)

Fork of `vishvananda/netlink`, with some rule fixes.

The library needs to be updated with the upstream.

#### quic-go

Link: [GitHub repository](https://github.com/SagerNet/quic-go)

Fork of `lucas-clemente/quic-go`  and `HyNetwork/quic-go`, contains quic flow control and other fixes used by Hysteria.

Since the author of Hysteria does not follow the upstream updates in time, and the provided fork needs to use replace,
we need to do this.

The library needs to be updated with the upstream.

#### certmagic

Link: [GitHub repository](https://github.com/SagerNet/certmagic)

Fork of `caddyserver/certmagic`

Since upstream uses `miekg/dns` and we use `x/net/dnsmessage`, we need to replace its DNS part with our own
implementation.

The library needs to be updated with the upstream.

#### smux

Link: [GitHub repository](https://github.com/SagerNet/smux)

Fork of `xtaci/smux`

Modify the code to support the writev it uses internally and unify the buffer pool, which prevents it from allocating
64k buffers for per connection and improves performance.

Upstream doesn't seem to be updated anymore, maybe a replacement is needed.

Note: while yamux is still actively maintained and better known, it seems to be less performant.
