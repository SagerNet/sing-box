!!! warning ""

    It's a proprietary protocol created by SagerNet, not part of shadowsocks.

The UDP over TCP protocol is used to transmit UDP packets in TCP.

### Structure

```json
{
  "enabled": true,
  "version": 2
}
```

!!! info ""

    The structure can be replaced with a boolean value when the version is not specified.

### Fields

#### enabled

Enable the UDP over TCP protocol.

#### version

The protocol version, `1` or `2`.

2 is used by default.

### Application support

| Project      | UoT v1               | UoT v2               |
|--------------|----------------------|----------------------|
| sing-box     | v0 (2022/08/11)      | v1.2-beta9           |
| Clash.Meta   | v1.12.0 (2022/07/02) | v1.14.3 (2023/03/31) |
| Shadowrocket | v2.2.12 (2022/08/13) | /                    |

### Protocol details

#### Protocol version 1

The client requests the magic address to the upper layer proxy protocol to indicate the request: `sp.udp-over-tcp.arpa`

#### Stream format

| ATYP | address  | port  | length | data     |
|------|----------|-------|--------|----------|
| u8   | variable | u16be | u16be  | variable |

**ATYP / address / port**: Uses the SOCKS address format, but with different address types:

| ATYP   | Address type |
|--------|--------------|
| `0x00` | IPv4 Address |
| `0x01` | IPv6 Address |
| `0x02` | Domain Name  |

#### Protocol version 2

Protocol version 2 uses a new magic address: `sp.v2.udp-over-tcp.arpa`

##### Request format

| isConnect | ATYP | address  | port  |
|-----------|------|----------|-------|
| u8        | u8   | variable | u16be |

**isConnect**: Set to 1 to indicates that the stream uses the connect format, 0 to disable.

**ATYP / address / port**: Request destination, uses the SOCKS address format.

##### Connect stream format

| length | data     |
|--------|----------|
| u16be  | variable |

##### Non-connect stream format

As the same as the stream format in protocol version 1.