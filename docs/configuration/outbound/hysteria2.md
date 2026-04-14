!!! quote "Changes in sing-box 1.14.0"

    :material-plus: [hop_interval_max](#hop_interval_max)  
    :material-plus: [bbr_profile](#bbr_profile)

!!! quote "Changes in sing-box 1.11.0"

    :material-plus: [server_ports](#server_ports)  
    :material-plus: [hop_interval](#hop_interval)

### Structure

```json
{
  "type": "hysteria2",
  "tag": "hy2-out",

  "server": "127.0.0.1",
  "server_port": 1080,
  "server_ports": [
    "2080:3000"
  ],
  "hop_interval": "",
  "hop_interval_max": "",
  "up_mbps": 100,
  "down_mbps": 100,
  "obfs": {
    "type": "salamander",
    "password": "cry_me_a_r1ver"
  },
  "password": "goofy_ahh_password",
  "network": "tcp",
  "tls": {},

  ... // QUIC Fields

  "bbr_profile": "",
  "brutal_debug": false,

  ... // Dial Fields
}
```

!!! note ""

    You can ignore the JSON Array [] tag when the content is only one item

!!! warning "Difference from official Hysteria2"

    The official Hysteria2 supports an authentication method called **userpass**,
    which essentially uses a combination of `<username>:<password>` as the actual password,
    while sing-box does not provide this alias.
    If you are planning to use sing-box with the official program,
    please note that you will need to fill the combination as the password.

### Fields

#### server

==Required==

The server address.

#### server_port

==Required==

The server port.

Ignored if `server_ports` is set.

#### server_ports

!!! question "Since sing-box 1.11.0"

Server port range list.

Conflicts with `server_port`.

#### hop_interval

!!! question "Since sing-box 1.11.0"

Port hopping interval.

`30s` is used by default.

#### hop_interval_max

!!! question "Since sing-box 1.14.0"

Maximum port hopping interval, used for randomization.

If set, the actual hop interval will be randomly chosen between `hop_interval` and `hop_interval_max`.

#### up_mbps, down_mbps

Max bandwidth, in Mbps.

If empty, the BBR congestion control algorithm will be used instead of Hysteria CC.

#### obfs.type

QUIC traffic obfuscator type, only available with `salamander`.

Disabled if empty.

#### obfs.password

QUIC traffic obfuscator password.

#### password

Authentication password.

#### network

Enabled network

One of `tcp` `udp`.

Both is enabled by default.

#### tls

==Required==

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

### QUIC Fields

See [QUIC Fields](/configuration/shared/quic/) for details.

#### bbr_profile

!!! question "Since sing-box 1.14.0"

BBR congestion control algorithm profile, one of `conservative` `standard` `aggressive`.

`standard` is used by default.

#### brutal_debug

Enable debug information logging for Hysteria Brutal CC.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
