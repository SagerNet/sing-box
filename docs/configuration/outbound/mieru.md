---
icon: material/new-box
---

### Structure

```json
{
  "type": "mieru",
  "tag": "mieru-out",

  "server": "127.0.0.1",
  "server_port": 1080,
  "server_ports": [
    "9000-9010",
    "9020-9030"
  ],
  "transport": "TCP",
  "username": "asdf",
  "password": "hjkl",
  "multiplexing": "MULTIPLEXING_LOW",

  ... // Dial Fields
}
```

### Fields

#### server

==Required==

The server address.

#### server_port

The server port.

Must set at least one field between `server_port` and `server_ports`.

#### server_ports

Server port range list.

Must set at least one field between `server_port` and `server_ports`.

#### transport

==Required==

Transmission protocol. The only allowed value is `TCP`.

#### username

==Required==

mieru user name.

#### password

==Required==

mieru password.

#### multiplexing

Multiplexing level. Supported values are `MULTIPLEXING_OFF`, `MULTIPLEXING_LOW`, `MULTIPLEXING_MIDDLE`, `MULTIPLEXING_HIGH`. `MULTIPLEXING_OFF` disables multiplexing.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
