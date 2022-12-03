```json
{
  "dns": {
    "rules": [
      {
        "domain": [
          "clash.razord.top",
          "yacd.haishan.me"
        ],
        "server": "local"
      },
      {
        "clash_mode": "direct",
        "server": "local"
      }
    ]
  },
  "outbounds": [
    {
      "type": "selector",
      "tag": "default",
      "outbounds": [
        "proxy-a",
        "proxy-b"
      ]
    }
  ],
  "route": {
    "rules": [
      {
        "clash_mode": "direct",
        "outbound": "direct"
      },
      {
        "domain": [
          "clash.razord.top",
          "yacd.haishan.me"
        ],
        "outbound": "direct"
      }
    ],
    "final": "default"
  },
  "experimental": {
    "clash_api": {
      "external_controller": "127.0.0.1:9090",
      "store_selected": true
    }
  }
}

```