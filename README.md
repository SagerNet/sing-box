# sing-box

A fork of sing-box with [mieru](https://github.com/enfein/mieru) protocol
support.

## Example Configuration

```js
{
    "inbounds": [
        {
            "type": "mixed",
            "tag": "mixed-in",
            "listen": "0.0.0.0",
            "listen_port": 1080
        }
    ],
    "outbounds": [
        {
            "type": "mieru",
            "tag": "mieru-out",
            "server": "127.0.0.1",
            "server_port": 8964,
            "transport": "TCP",
            "username": "baozi",
            "password": "manlianpenfen"
        }
    ],
    "route": {
        "rules": [
            {
                "inbound": ["mixed-in"],
                "action": "route",
                "outbound": "mieru-out"
            }
        ]
    },
    "log": {
        "level": "warn"
    }
}
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

In addition, no derivative work may use the name or imply association
with this application without prior consent.
```
