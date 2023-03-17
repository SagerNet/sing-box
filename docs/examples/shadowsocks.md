# Shadowsocks

!!! warning ""

    For censorship bypass usage in China, we recommend using UDP over TCP and disabling UDP on the server.

## Single User

#### Server

```json
{
  "inbounds": [
    {
      "type": "shadowsocks",
      "listen": "::",
      "listen_port": 8080,
      "network": "tcp",
      "method": "2022-blake3-aes-128-gcm",
      "password": "8JCsPssfgS8tiRwiMlhARg=="
    }
  ]
}
```

#### Client

```json
{
  "inbounds": [
    {
      "type": "mixed",
      "listen": "::",
      "listen_port": 2080
    }
  ],
  "outbounds": [
    {
      "type": "shadowsocks",
      "server": "127.0.0.1",
      "server_port": 8080,
      "method": "2022-blake3-aes-128-gcm",
      "password": "8JCsPssfgS8tiRwiMlhARg==",
      "udp_over_tcp": true
    }
  ]
}

```

## Multiple Users

#### Server

```json
{
  "inbounds": [
    {
      "type": "shadowsocks",
      "listen": "::",
      "listen_port": 8080,
      "method": "2022-blake3-aes-128-gcm",
      "password": "8JCsPssfgS8tiRwiMlhARg==",
      "users": [
        {
          "name": "sekai",
          "password": "BXYxVUXJ9NgF7c7KPLQjkg=="
        }
      ]
    }
  ]
}
```

#### Client

```json
{
  "inbounds": [
    {
      "type": "mixed",
      "listen": "::",
      "listen_port": 2080
    }
  ],
  "outbounds": [
    {
      "type": "shadowsocks",
      "server": "127.0.0.1",
      "server_port": 8080,
      "method": "2022-blake3-aes-128-gcm",
      "password": "8JCsPssfgS8tiRwiMlhARg==:BXYxVUXJ9NgF7c7KPLQjkg=="
    }
  ]
}

```

## Relay

#### Server

```json
{
  "inbounds": [
    {
      "type": "shadowsocks",
      "listen": "::",
      "listen_port": 8080,
      "method": "2022-blake3-aes-128-gcm",
      "password": "8JCsPssfgS8tiRwiMlhARg=="
    }
  ]
}
```

#### Relay

```json
{
  "inbounds": [
    {
      "type": "shadowsocks",
      "listen": "::",
      "listen_port": 8081,
      "method": "2022-blake3-aes-128-gcm",
      "password": "BXYxVUXJ9NgF7c7KPLQjkg==",
      "destinations": [
        {
          "name": "my_server",
          "password": "8JCsPssfgS8tiRwiMlhARg==",
          "server": "127.0.0.1",
          "server_port": 8080
        }
      ]
    }
  ]
}
```

#### Client

```json
{
  "inbounds": [
    {
      "type": "mixed",
      "listen": "::",
      "listen_port": 2080
    }
  ],
  "outbounds": [
    {
      "type": "shadowsocks",
      "server": "127.0.0.1",
      "server_port": 8081,
      "method": "2022-blake3-aes-128-gcm",
      "password": "8JCsPssfgS8tiRwiMlhARg==:BXYxVUXJ9NgF7c7KPLQjkg=="
    }
  ]
}

```