#### Server

| Feature                                                    | v2ray-core | clash |
|------------------------------------------------------------|------------|-------|
| Direct inbound                                             | ✔          | X     |
| SOCKS4a inbound                                            | ✔          | X     |
| Mixed (http/socks) inbound                                 | X          | ✔     |
| Shadowsocks AEAD 2022 single-user/multi-user/relay inbound | X          | X     |
| VMess/Trojan inbound                                       | ✔          | X     |
| Naive/Hysteria inbound                                     | X          | X     |
| Resolve incoming domain names using custom policy          | X          | X     |
| Set system proxy on Windows/Linux/macOS/Android            | X          | X     |
| TLS certificate auto reload                                | X          | X     |
| TLS ACME certificate issuer                                | X          | X     |

#### Client

| Feature                                                | v2ray-core                         | clash    |
|--------------------------------------------------------|------------------------------------|----------|
| Set upstream client (proxy chain)                      | TCP only, and has poor performance | TCP only |
| Bind to network interface                              | Linux only                         | ✔        |
| Custom dns strategy for resolving server address       | X                                  | X        |
| Fast fallback (RFC 6555) support for connect to server | X                                  | X        |
| SOCKS4/4a outbound                                     | added by me                        | X        |
| Shadowsocks AEAD 2022 outbound                         | X                                  | X        |
| Shadowsocks UDP over TCP                               | X                                  | X        |
| Multiplex (smux/yamux)                                 | mux.cool                           | X        |
| Tor/WireGuard/Hysteria outbound                        | X                                  | X        |
| Selector outbound and Clash API                        | X                                  | ✔        |

#### Sniffing

| Protocol         | v2ray-core  | clash-premium |
|------------------|-------------|---------------|
| HTTP Host        | ✔           | X             |
| QUIC ClientHello | added by me | added by me   |
| STUN             | X           | X             |

| Feature                                 | v2ray-core                | clash-premium |
|-----------------------------------------|---------------------------|---------------|
| For routing only                        | added by me               | X             |
| No performance impact (like TCP splice) | no general splice support | X             |
| Set separately for each server          | ✔                         | X             |

#### Routing

| Feature                    | v2ray-core | clash-premium |
|----------------------------|------------|---------------|
| Auto detect interface      | X          | tun only      |
| Set default interface name | X          | tun only      |
| Set default firewall mark  | X          | X             |

#### Routing Rule

| Rule                 | v2ray-core                 | clash |
|----------------------|----------------------------|-------|
| Inbound              | ✔                          | X     |
| IP Version           | X                          | X     |
| User from inbound    | vmess and shadowsocks only | X     |
| Sniffed protocol     | ✔                          | X     |
| GeoSite              | ✔                          | X     |
| Process name         | X                          | ✔     |
| Android package name | X                          | X     |
| Linux user/user id   | X                          | X     |
| Invert rule          | X                          | X     |
| Logical rule         | X                          | X     |

#### DNS

| Feature                            | v2ray-core  | clash |
|------------------------------------|-------------|-------|
| DNS proxy                          | A/AAAA only | ✔     |
| DNS cache                          | A/AAAA only | X     |
| DNS routing                        | X           | X     |
| DNS Over QUIC                      | ✔           | X     |
| DNS Over HTTP3                     | X           | X     |
| Fake dns response with custom code | X           | X     |

#### Tun

| Feature                                   | clash-premium |
|-------------------------------------------|---------------|
| Full IPv6 support                         | X             |
| Auto route on Linux/Windows/maxOS/Android | ✔             |
| Embed windows driver                      | X             |
| Custom address/mtu                        | X             |
| Limit uid (Linux) in routing              | X             |
| Limit android user in routing             | X             |
| Limit android package in routing          | X             |

#### Memory usage

| GeoSite code      | sing-box | v2ray-core |
|-------------------|----------|------------|
| cn                | 17.8M    | 140.3M     |
| cn (Loyalsoldier) | 74.3M    | 246.7M     |

#### Shadowsocks benchmark

| /                                  |   none    | aes-128-gcm | 2022-blake3-aes-128-gcm |
|------------------------------------|:---------:|:-----------:|:-----------------------:|
| v2ray-core (5.0.7)                 | 13.0 Gbps |  5.02 Gbps  |            /            |
| shadowsocks-rust (v1.15.0-alpha.5) | 10.7 Gbps |      /      |        9.36 Gbps        |
| sing-box                           | 29.0 Gbps |      /      |        11.8 Gbps        |

#### License

| /          | License                           |
|------------|-----------------------------------|
| sing-box   | GPLv3 or later (Full open-source) |
| v2ray-core | MIT (Full open-source)            |
| clash      | GPLv3                             |