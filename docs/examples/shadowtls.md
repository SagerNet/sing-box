#### Server

```json
{
  "inbounds": [
    {
      "type": "shadowtls",
      "listen": "::",
      "listen_port": 4443,
      "version": 3,
      "users": [
        {
          "name": "sekai",
          "password": "8JCsPssfgS8tiRwiMlhARg=="
        }
      ],
      "handshake": {
        "server": "google.com",
        "server_port": 443
      },
      "detour": "shadowsocks-in"
    },
    {
      "type": "shadowsocks",
      "tag": "shadowsocks-in",
      "listen": "127.0.0.1",
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
  "outbounds": [
    {
      "type": "shadowsocks",
      "method": "2022-blake3-aes-128-gcm",
      "password": "8JCsPssfgS8tiRwiMlhARg==",
      "detour": "shadowtls-out",
      "multiplex": {
        "enabled": true,
        "max_connections": 4,
        "min_streams": 4
      }
      // or "udp_over_tcp": true
    },
    {
      "type": "shadowtls",
      "tag": "shadowtls-out",
      "server": "127.0.0.1",
      "server_port": 4443,
      "version": 3,
      "password": "8JCsPssfgS8tiRwiMlhARg==",
      "tls": {
        "enabled": true,
        "server_name": "google.com",
        "utls": {
          "enabled": true,
          "fingerprint": "chrome"
        }
      }
    }
  ]
}
```
