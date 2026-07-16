!!! quote "Changes in sing-box 1.14.0"

    :material-plus: [udp_mapping](/configuration/shared/udp-nat/#udp_mapping)  
    :material-plus: [udp_filtering](/configuration/shared/udp-nat/#udp_filtering)  
    :material-plus: [udp_nat_max](/configuration/shared/udp-nat/#udp_nat_max)

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

  ... // UDP NAT Fields

  "workers": 0,
 
  ... // Dial Fields
}
```

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

#### workers

WireGuard worker count.

CPU count is used by default.

### UDP NAT Fields

See [UDP NAT Fields](/configuration/shared/udp-nat/) for details.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
