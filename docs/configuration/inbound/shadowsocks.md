### Structure

```json
{
  "type": "shadowsocks",
  "tag": "ss-in",

  ... // Listen Fields

  "method": "2022-blake3-aes-128-gcm",
  "password": "8JCsPssfgS8tiRwiMlhARg==",
  "multiplex": {}
}
```

### Multi-User Structure

```json
{
  "method": "2022-blake3-aes-128-gcm",
  "password": "8JCsPssfgS8tiRwiMlhARg==",
  "users": [
    {
      "name": "sekai",
      "password": "PCD2Z4o12bKUoFa3cC97Hw=="
    }
  ],
  "multiplex": {}
}
```

### Relay Structure

```json
{
  "type": "shadowsocks",
  "method": "2022-blake3-aes-128-gcm",
  "password": "8JCsPssfgS8tiRwiMlhARg==",
  "destinations": [
    {
      "name": "test",
      "server": "example.com",
      "server_port": 8080,
      "password": "PCD2Z4o12bKUoFa3cC97Hw=="
    }
  ],
  "multiplex": {}
}
```

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.

### Fields

#### network

Listen network, one of `tcp` `udp`.

Both if empty.

#### method

==Required==

| Method                        | Key Length |
|-------------------------------|------------|
| 2022-blake3-aes-128-gcm       | 16         |
| 2022-blake3-aes-256-gcm       | 32         |
| 2022-blake3-chacha20-poly1305 | 32         |
| none                          | /          |
| aes-128-gcm                   | /          |
| aes-192-gcm                   | /          |
| aes-256-gcm                   | /          |
| chacha20-ietf-poly1305        | /          |
| xchacha20-ietf-poly1305       | /          |

#### password

==Required==

| Method        | Password Format                                |
|---------------|------------------------------------------------|
| none          | /                                              |
| 2022 methods  | `sing-box generate rand --base64 <Key Length>` |
| other methods | any string                                     |

#### multiplex

See [Multiplex](/configuration/shared/multiplex#inbound) for details.
