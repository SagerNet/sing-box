# Shadowsocks Client

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
      "server": "::",
      "server_port": 8080,
      "method": "2022-blake3-aes-128-gcm",
      "password": "8JCsPssfgS8tiRwiMlhARg=="
    }
  ]
}

```