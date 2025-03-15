!!! quote "Changes in sing-box 1.10.0"

    :material-plus: QUIC client type detect support for QUIC  
    :material-plus: Chromium support for QUIC  
    :material-plus: BitTorrent support  
    :material-plus: DTLS support  
    :material-plus: SSH support  
    :material-plus: RDP support

If enabled in the inbound, the protocol and domain name (if present) of by the connection can be sniffed.

#### Supported Protocols

| Network |   Protocol   | Domain Name |      Client      |
|:-------:|:------------:|:-----------:|:----------------:|
|   TCP   |    `http`    |    Host     |        /         |
|   TCP   |    `tls`     | Server Name |        /         |
|   UDP   |    `quic`    | Server Name | QUIC Client Type |
|   UDP   |    `stun`    |      /      |        /         |
| TCP/UDP |    `dns`     |      /      |        /         |
| TCP/UDP | `bittorrent` |      /      |        /         |
|   UDP   |    `dtls`    |      /      |        /         |
|   TCP   |    `ssh`     |      /      | SSH Client Name  |
|   TCP   |    `rdp`     |      /      |        /         |
|   UDP   |    `ntp`     |      /      |        /         |

|       QUIC Client        |    Type    |
|:------------------------:|:----------:|
|     Chromium/Cronet      | `chromium` |
| Safari/Apple Network API |  `safari`  |
| Firefox / uquic firefox  | `firefox`  |
|  quic-go / uquic chrome  | `quic-go`  |
