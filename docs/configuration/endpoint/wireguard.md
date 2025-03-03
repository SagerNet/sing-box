---
icon: material/new-box
---

!!! question "Since sing-box 1.11.0"

### Structure

```json
{
  "type": "wireguard",
  "tag": "wg-ep",
  
  "system": false,
  "name": "",
  "mtu": 1408,
  "address": [],
  "private_key": "",
  "listen_port": 10000,
  "peers": [
    {
      "address": "127.0.0.1",
      "port": 10001,
      "public_key": "",
      "pre_shared_key": "",
      "allowed_ips": [],
      "persistent_keepalive_interval": 0,
      "reserved": [0, 0, 0]
    }
  ],
  "udp_timeout": "",
  "workers": 0,
 
  ... // Dial Fields
}
```

!!! question "Since sing-box 1.xx.0"

#### Advanced security (AmneziaWG)

To activate advanced security mode for WireGuard (powered by AmneziaWG), please add the following fields to the configuration:

```json5
{
  ...
  "private_key": "....",
  "jc": 0, // JunkPacketCount           
  "jmin": 0, // JunkPacketMinSize         
  "jmax":  0, // JunkPacketMaxSize         
  "s1": 0, // InitPacketJunkSize        
  "s2":  0, // ResponsePacketJunkSize    
  "h1": 0, // InitPacketMagicHeader     
  "h2":  0, // ResponsePacketMagicHeader 
  "h3": 0, // UnderloadPacketMagicHeader
  "h4":  0, // TransportPacketMagicHeader
}
```

Setting any of these values with a non-zero value will activate the corresponding security feature.
If neither of these values is set, the default WireGuard security will be used.

!!! note ""

    You can ignore the JSON Array [] tag when the content is only one item

### Fields

#### system

Use system interface.

Requires privilege and cannot conflict with exists system interfaces.

#### name

Custom interface name for system interface.

#### mtu

WireGuard MTU.

`1408` will be used by default.

#### address

==Required==

List of IP (v4 or v6) address prefixes to be assigned to the interface.

#### private_key

==Required==

WireGuard requires base64-encoded public and private keys. These can be generated using the wg(8) utility:

```shell
wg genkey
echo "private key" || wg pubkey
```

or `sing-box generate wg-keypair`.

#### peers

==Required==

List of WireGuard peers.

#### peers.address

WireGuard peer address.

#### peers.port

WireGuard peer port.

#### peers.public_key

==Required==

WireGuard peer public key.

#### peers.pre_shared_key

WireGuard peer pre-shared key.

#### peers.allowed_ips

==Required==

WireGuard allowed IPs.

#### peers.persistent_keepalive_interval

WireGuard persistent keepalive interval, in seconds.

Disabled by default.

#### peers.reserved

WireGuard reserved field bytes.

#### udp_timeout

UDP NAT expiration time.

`5m` will be used by default.

#### workers

WireGuard worker count.

CPU count is used by default.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
