# Home

Welcome to the wiki page for the sing-box project.

The universal proxy platform.

## Installation

sing-box requires Golang 1.18 or a higher version.

```bash
go install -v github.com/sagernet/sing-box/cmd/sing-box@latest
```

Install with options:

```bash
go install -v -tags "with_clash_api,no_gvisor" github.com/sagernet/sing-box/cmd/sing-box@latest
```

| Build Tag        | Description                                                                                             |
|------------------|---------------------------------------------------------------------------------------------------------|
| `with_quic`      | Build with quic support, which required by [QUIC and HTTP3](./configuration/dns/server) dns transports. |
| `with_clash_api` | Build with clash api support, see [Experimental](./configuration/experimental#clash-api-fields).        |
| `no_gvisor`      | Build without gVisor, which required by the [Tun](./configuration/inbound/tun) inbound.                 |

The binary is built under $GOPATH/bin

```bash
sing-box version
```

## License

```
Copyright (C) 2022 by nekohasekai <contact-sagernet@sekai.icu>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
```
