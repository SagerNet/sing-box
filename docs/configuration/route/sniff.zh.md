!!! quote "sing-box 1.10.0 中的更改"

    :material-plus: QUIC 的 客户端类型探测支持  
    :material-plus: QUIC 的 Chromium 支持  
    :material-plus: BitTorrent 支持  
    :material-plus: DTLS 支持  
    :material-plus: SSH 支持  
    :material-plus: RDP 支持

[`sniff`](/zh/configuration/route/rule_action/#sniff) 路由规则动作可以嗅探连接的协议和域名（如果存在）。

#### 支持的协议

|   网络    |      协议      |     域名      |    客户端     |
|:-------:|:------------:|:-----------:|:----------:|
|   TCP   |    `http`    |    Host     |     /      |
|   TCP   |    `tls`     | Server Name |     /      |
|   UDP   |    `quic`    | Server Name | QUIC 客户端类型 |
|   UDP   |    `stun`    |      /      |     /      |
| TCP/UDP |    `dns`     |      /      |     /      |
| TCP/UDP | `bittorrent` |      /      |     /      |
|   UDP   |    `dtls`    |      /      |     /      |
|   TCP   |    `ssh`     |      /      | SSH 客户端名称  |
|   TCP   |    `rdp`     |      /      |     /      |
|   UDP   |    `ntp`     |      /      |     /      |

|         QUIC 客户端         |     类型     |
|:------------------------:|:----------:|
|     Chromium/Cronet      | `chromium` |
| Safari/Apple Network API |  `safari`  |
| Firefox / uquic firefox  | `firefox`  |
|  quic-go / uquic chrome  | `quic-go`  |
