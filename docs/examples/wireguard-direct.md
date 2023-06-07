# WireGuard Direct

```json
{
  "dns": {
    "servers": [
      {
        "tag": "google",
        "address": "tls://8.8.8.8"
      },
      {
        "tag": "local",
        "address": "223.5.5.5",
        "detour": "direct"
      }
    ],
    "rules": [
      {
        "geoip": "cn",
        "server": "direct"
      }
    ],
    "reverse_mapping": true
  },
  "inbounds": [
    {
      "type": "tun",
      "tag": "tun",
      "inet4_address": "172.19.0.1/30",
      "auto_route": true,
      "sniff": true,
      "stack": "system"
    }
  ],
  "outbounds": [
    {
      "type": "wireguard",
      "tag": "wg",
      "server": "127.0.0.1",
      "server_port": 2345,
      "local_address": [
        "172.19.0.1/128"
      ],
      "private_key": "KLTnpPY03pig/WC3zR8U7VWmpANHPFh2/4pwICGJ5Fk=",
      "peer_public_key": "uvNabcamf6Rs0vzmcw99jsjTJbxo6eWGOykSY66zsUk="
    },
    {
      "type": "dns",
      "tag": "dns"
    },
    {
      "type": "direct",
      "tag": "direct"
    },
    {
      "type": "block",
      "tag": "block"
    }
  ],
  "route": {
    "ip_rules": [
      {
        "port": 53,
        "action": "return"
      },
      {
        "geoip": "cn",
        "geosite": "cn",
        "action": "return"
      },
      {
        "action": "direct",
        "outbound": "wg"
      }
    ],
    "rules": [
      {
        "protocol": "dns",
        "outbound": "dns"
      },
      {
        "geoip": "cn",
        "geosite": "cn",
        "outbound": "direct"
      }
    ],
    "auto_detect_interface": true
  }
}
```