### Structure

```json
{
  "type": "shadowsocksr",
  "tag": "ssr-out",
  
  "server": "127.0.0.1",
  "server_port": 1080,
  "method": "aes-128-cfb",
  "password": "8JCsPssfgS8tiRwiMlhARg==",
  "obfs": "plain",
  "obfs_param": "",
  "protocol": "origin",
  "protocol_param": "",
  "network": "udp",

  ... // Dial Fields
}
```

!!! warning ""

    The ShadowsocksR protocol is obsolete and unmaintained. This outbound is provided for compatibility only.

!!! warning ""

    ShadowsocksR is not included by default, see [Installation](/#installation).

### Fields

#### server

==Required==

The server address.

#### server_port

==Required==

The server port.

#### method

==Required==

Encryption methods:

* `aes-128-ctr`
* `aes-192-ctr`
* `aes-256-ctr`
* `aes-128-cfb`
* `aes-192-cfb`
* `aes-256-cfb`
* `rc4-md5`
* `chacha20-ietf`
* `xchacha20`

#### password

==Required==

The shadowsocks password.

#### obfs

The ShadowsocksR obfuscate.

* plain
* http_simple
* http_post
* random_head
* tls1.2_ticket_auth

#### obfs_param

The ShadowsocksR obfuscate parameter.

#### protocol

The ShadowsocksR protocol.

* origin
* verify_sha1
* auth_sha1_v4
* auth_aes128_md5
* auth_aes128_sha1
* auth_chain_a
* auth_chain_b

#### protocol_param

The ShadowsocksR protocol parameter.

#### network

Enabled network

One of `tcp` `udp`.

Both is enabled by default.

### Dial Fields

See [Dial Fields](/configuration/shared/dial) for details.
