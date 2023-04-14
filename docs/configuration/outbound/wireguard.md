### Structure

```json
{
  "type": "wireguard",
  "tag": "wireguard-out",
  
  "server": "127.0.0.1",
  "server_port": 1080,
  "system_interface": false,
  "interface_name": "wg0",
  "local_address": [
    "10.0.0.2/32"
  ],
  "private_key": "YNXtAzepDqRv9H52osJVDQnznT5AM11eCK3ESpwSt04=",
  "peers": [
    {
      "server": "127.0.0.1",
      "server_port": 1080,
      "public_key": "Z1XXLsKYkYxuiYjJIkRvtIKFepCYHTgON+GwPq7SOV4=",
      "pre_shared_key": "31aIhAPwktDGpH4JDhA8GNvjFXEf/a6+UaQRyOAiyfM=",
      "allowed_ips": [
        "0.0.0.0/0"
      ],
      "reserved": [0, 0, 0]
    }
  ],
  "peer_public_key": "Z1XXLsKYkYxuiYjJIkRvtIKFepCYHTgON+GwPq7SOV4=",
  "pre_shared_key": "31aIhAPwktDGpH4JDhA8GNvjFXEf/a6+UaQRyOAiyfM=",
  "reserved": [0, 0, 0],
  "workers": 4,
  "mtu": 1408,
  "network": "tcp",

  ... // Dial Fields
}
```

!!! warning ""

    WireGuard is not included by default, see [Installation](/#installation).

!!! warning ""

    gVisor, which is required by the unprivileged WireGuard is not included by default, see [Installation](/#installation).

### Fields

#### server

==Required if multi-peer disabled==

The server address.

#### server_port

==Required if multi-peer disabled==

The server port.

#### system_interface

Use system tun support.

Requires privilege and cannot conflict with system interfaces.

Forced if gVisor not included in the build.

#### interface_name

Custom device name when `system_interface` enabled.

#### local_address

==Required==

List of IP (v4 or v6) address prefixes to be assigned to the interface.

#### private_key

==Required==

WireGuard requires base64-encoded public and private keys. These can be generated using the wg(8) utility:

```shell
wg genkey
echo "private key" || wg pubkey
```

#### peers

Multi-peer support. 

If enabled, `server, server_port, peer_public_key, pre_shared_key` will be ignored.

#### peers.allowed_ips

WireGuard allowed IPs.

#### peers.reserved

WireGuard reserved field bytes.

`$outbound.reserved` will be used if empty.

#### peer_public_key

==Required if multi-peer disabled==

WireGuard peer public key.

#### pre_shared_key

WireGuard pre-shared key.

#### reserved

WireGuard reserved field bytes.

#### workers

WireGuard worker count.

CPU count is used by default.

#### mtu

WireGuard MTU.

1408 will be used if empty.

#### network

Enabled network

One of `tcp` `udp`.

Both is enabled by default.

### Dial Fields

See [Dial Fields](/configuration/shared/dial) for details.
