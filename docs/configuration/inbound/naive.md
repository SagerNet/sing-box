!!! quote "Changes in sing-box 1.13.0"

    :material-plus: [quic_congestion_control](#quic_congestion_control)

### Structure

```json
{
"type": "naive",
"tag": "naive-in",
"network": "udp",
...
// Listen Fields

"users": [
{
"username": "sekai",
"password": "password"
}
],
"quic_congestion_control": "",
"tls": {}
}
```

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.

### Fields

#### network

Listen network, one of `tcp` `udp`.

Both if empty.

#### users

==Required==

Naive users.

#### quic_congestion_control

!!! question "Since sing-box 1.13.0"

QUIC congestion control algorithm.

| Algorithm      | Description                     |
|----------------|---------------------------------|
| `bbr`          | BBR                             |
| `cubic`        | CUBIC                           |
| `reno`         | New Reno                        |

`bbr` is used by default.

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#inbound).