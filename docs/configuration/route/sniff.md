---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.10.0"

    :material-plus: BitTorrent support
    :material-plus: DTLS support

If enabled in the inbound, the protocol and domain name (if present) of by the connection can be sniffed.

#### Supported Protocols

| Network |   Protocol   | Domain Name |
|:-------:|:------------:|:-----------:|
|   TCP   |    `http`    |    Host     |
|   TCP   |    `tls`     | Server Name |
|   UDP   |    `quic`    | Server Name |
|   UDP   |    `stun`    |      /      |
| TCP/UDP |    `dns`     |      /      |
| TCP/UDP | `bittorrent` |      /      |
|   UDP   |    `dtls`    |      /      |
