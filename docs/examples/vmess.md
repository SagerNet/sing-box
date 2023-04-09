# VMess

## Websocket

```json
{
  "inbounds": [
    {
			"type": "vmess",
			"tag": "vmess-in",
			"listen": "::",
			"listen_port": 8080,
			"tcp_fast_open": false,
			"sniff": false,
			"sniff_override_destination": false,
			"domain_strategy": "prefer_ipv4",
			"proxy_protocol": false,
			"users": [
				{
					"name": "sekai",
					"uuid": "e70945d9-47f5-4ebf-9c48-d96ca91cfe3e",
					"alterId": 0
				}
			],
			"transport": {
				"type": "ws",
				"path": "/"
			}
		}
  ]
}
```