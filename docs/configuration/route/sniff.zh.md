---
icon: material/new-box
---

!!! quote "sing-box 1.10.0 中的更改"

    :material-plus: BitTorrent 支持
    :material-plus: DTLS 支持

如果在入站中启用，则可以嗅探连接的协议和域名（如果存在）。

#### 支持的协议

|   网络    |      协议      |     域名      |
|:-------:|:------------:|:-----------:|
|   TCP   |    `http`    |    Host     |
|   TCP   |    `tls`     | Server Name |
|   UDP   |    `quic`    | Server Name |
|   UDP   |    `stun`    |      /      |
| TCP/UDP |    `dns`     |      /      |
| TCP/UDP | `bittorrent` |      /      |
|   UDP   |    `dtls`    |      /      |
