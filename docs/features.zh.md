#### 服务端

| 特性                                          | v2ray-core | clash |
| --------------------------------------------- | ---------- | ----- |
| Direct 入站                                   | ✔          | X     |
| SOCKS4a 入站                                  | ✔          | X     |
| Mixed (http/socks) 入站                       | X          | ✔     |
| Shadowsocks AEAD 2022 单用户/多用户/中继 入站 | X          | X     |
| VMess/Trojan 入站                             | ✔          | X     |
| Naive/Hysteria 入站                           | X          | X     |
| 使用自定义策略解析传入域名                    | X          | X     |
| 为 Windows/Linux/macOS/Android 设定系统代理   | X          | X     |
| 自动重载 TLS 证书                             | X          | X     |
| TLS ACME 证书签发工具                         | X          | X     |

#### 客户端

| 特性                                   | v2ray-core           | clash    |
| -------------------------------------- | -------------------- | -------- |
| 设置前置客户端（代理链）               | 仅限 TCP，且性能极低 | 仅限 TCP |
| 绑定网络接口                           | 仅限 Linux           | ✔        |
| 自定义服务器地址解析策略               | X                    | X        |
| 连接到服务端时支持快速回落（RFC 6555） | X                    | X        |
| SOCKS4/4a 出站                         | 由我添加             | X        |
| Shadowsocks AEAD 2022 出站             | X                    | X        |
| Shadowsocks UDP over TCP               | X                    | X        |
| 多路复用（smux/yamux）                 | mux.cool             | X        |
| Tor/WireGuard/Hysteria 出站            | X                    | X        |
| Selector 出站 及 Clash API             | X                    | ✔        |

#### 嗅探

| 协议        | v2ray-core | clash-premium |
| ----------- | ---------- | ------------- |
| HTTP 主机名 | ✔          | X             |
| QUIC 握手包 | 由我添加   | 由我添加      |
| STUN        | X          | X             |

| 特性                        | v2ray-core         | clash-premium |
| --------------------------- | ------------------ | ------------- |
| 仅用于路由                  | 由我添加           | X             |
| 无性能影响（如 TCP splice） | 无通用 splice 支持 | X             |
| 独立设定每个服务端          | ✔                  | X             |

#### 路由

| 特性               | v2ray-core | clash-premium |
| ------------------ | ---------- | ------------- |
| 自动检测接口       | X          | 仅限 tun      |
| 设置默认接口       | X          | 仅限 tun      |
| 设置默认数据包标记 | X          | X             |

#### 路由规则

| 规则                 | v2ray-core                | clash |
| -------------------- | ------------------------- | ----- |
| 入站                 | ✔                         | X     |
| IP 版本              | X                         | X     |
| 入站用户             | 仅限 vmess 和 shadowsocks | X     |
| 嗅探协议             | ✔                         | X     |
| GeoSite              | ✔                         | X     |
| 进程名               | X                         | ✔     |
| Android 包名         | X                         | X     |
| Linux 用户 / 用户 ID | X                         | X     |
| 反转规则             | X                         | X     |
| 逻辑规则             | X                         | X     |

#### DNS

| 特性                | v2ray-core  | clash |
| ------------------- | ----------- | ----- |
| DNS 代理            | 仅限 A/AAAA | ✔     |
| DNS 缓存            | 仅限 A/AAAA | X     |
| DNS 路由            | X           | X     |
| DNS Over QUIC       | ✔           | X     |
| DNS Over HTTP3      | X           | X     |
| 伪造自定义 DNS 响应 | X           | X     |

#### Tun

| 特性                                 | clash-premium |
| ------------------------------------ | ------------- |
| 完整 IPv6 支持                       | X             |
| Linux/Windows/macOS/Android 自动路由 | ✔             |
| 集成 Windows 驱动                    | X             |
| 自定义 地址 / mtu                    | X             |
| 路由中指定 uid（Linux）              | X             |
| 路由中指定用户（android）            | X             |
| 路由中指定包名（android）            | X             |

#### 内存占用

| GeoSite 版本      | sing-box | v2ray-core |
| ----------------- | -------- | ---------- |
| cn                | 17.8M    | 140.3M     |
| cn (Loyalsoldier) | 74.3M    | 246.7M     |

#### Shadowsocks 跑分

| /                                  |   none    | aes-128-gcm | 2022-blake3-aes-128-gcm |
| ---------------------------------- | :-------: | :---------: | :---------------------: |
| v2ray-core (5.0.7)                 | 13.0 Gbps |  5.02 Gbps  |            /            |
| shadowsocks-rust (v1.15.0-alpha.5) | 10.7 Gbps |      /      |        9.36 Gbps        |
| sing-box                           | 29.0 Gbps |      /      |        11.8 Gbps        |

#### 协议

| /          | 协议                       |
| ---------- | -------------------------- |
| sing-box   | GPLv3 or later（完全开源） |
| v2ray-core | MIT（完全开源）            |
| clash      | GPLv3                      |
