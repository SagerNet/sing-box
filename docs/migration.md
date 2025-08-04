---
icon: material/arrange-bring-forward
---

## 1.12.0

### Migrate to new DNS server formats

DNS servers are refactored for better performance and scalability.

!!! info "References"

    [DNS Server](/configuration/dns/server/) /
    [Legacy DNS Server](/configuration/dns/server/legacy/)

=== "Local"

    === ":material-card-remove: Deprecated"
        
        ```json
        {
          "dns": {
            "servers": [
              {
                "address": "local"
              }
            ]
          }
        }
        ```
    
    === ":material-card-multiple: New"
    
        ```json
        {
          "dns": {
            "servers": [
              {
                "type": "local"
              }
            ]
          }
        }
        ```

=== "TCP"

    === ":material-card-remove: Deprecated"
    
        ```json
        {
          "dns": {
            "servers": [
              {
                "address": "tcp://1.1.1.1"
              }
            ]
          }
        }
        ```
    
    === ":material-card-multiple: New"
    
        ```json
        {
          "dns": {
            "servers": [
              {
                "type": "tcp",
                "server": "1.1.1.1"
              }
            ]
          }
        }
        ```

=== "UDP"

    === ":material-card-remove: Deprecated"
    
        ```json
        {
          "dns": {
            "servers": [
              {
                "address": "1.1.1.1"
              }
            ]
          }
        }
        ```
    
    === ":material-card-multiple: New"
    
        ```json
        {
          "dns": {
            "servers": [
              {
                "type": "udp",
                "server": "1.1.1.1"
              }
            ]
          }
        }
        ```

=== "TLS"

    === ":material-card-remove: Deprecated"
    
        ```json
        {
          "dns": {
            "servers": [
              {
                "address": "tls://1.1.1.1"
              }
            ]
          }
        }
        ```
    
    === ":material-card-multiple: New"
    
        ```json
        {
          "dns": {
            "servers": [
              {
                "type": "tls",
                "server": "1.1.1.1"
              }
            ]
          }
        }
        ```

=== "HTTPS"

    === ":material-card-remove: Deprecated"
    
        ```json
        {
          "dns": {
            "servers": [
              {
                "address": "https://1.1.1.1/dns-query"
              }
            ]
          }
        }
        ```
    
    === ":material-card-multiple: New"
    
        ```json
        {
          "dns": {
            "servers": [
              {
                "type": "https",
                "server": "1.1.1.1"
              }
            ]
          }
        }
        ```

=== "QUIC"

    === ":material-card-remove: Deprecated"
    
        ```json
        {
          "dns": {
            "servers": [
              {
                "address": "quic://1.1.1.1"
              }
            ]
          }
        }
        ```
    
    === ":material-card-multiple: New"
    
        ```json
        {
          "dns": {
            "servers": [
              {
                "type": "quic",
                "server": "1.1.1.1"
              }
            ]
          }
        }
        ```

=== "HTTP3"

    === ":material-card-remove: Deprecated"
    
        ```json
        {
          "dns": {
            "servers": [
              {
                "address": "h3://1.1.1.1/dns-query"
              }
            ]
          }
        }
        ```
    
    === ":material-card-multiple: New"
    
        ```json
        {
          "dns": {
            "servers": [
              {
                "type": "h3",
                "server": "1.1.1.1"
              }
            ]
          }
        }
        ```

=== "DHCP"

    === ":material-card-remove: Deprecated"
    
        ```json
        {
          "dns": {
            "servers": [
              {
                "address": "dhcp://auto"
              },
              {
                "address": "dhcp://en0"
              }
            ]
          }
        }
        ```
    
    === ":material-card-multiple: New"
    
        ```json
        {
          "dns": {
            "servers": [
              {
                "type": "dhcp",
              },
              {
                "type": "dhcp",
                "interface": "en0"
              }
            ]
          }
        }
        ```

=== "FakeIP"

    === ":material-card-remove: Deprecated"
        
        ```json
        {
          "dns": {
            "servers": [
              {
                "address": "1.1.1.1"
              },
              {
                "address": "fakeip",
                "tag": "fakeip"
              }
            ],
            "rules": [
              {
                "query_type": [
                  "A",
                  "AAAA"
                ],
                "server": "fakeip"
              }
            ],
            "fakeip": {
              "enabled": true,
              "inet4_range": "198.18.0.0/15",
              "inet6_range": "fc00::/18"
            }
          }
        }
        ```
    
    === ":material-card-multiple: New"
    
        ```json
        {
          "dns": {
            "servers": [
              {
                "type": "udp",
                "server": "1.1.1.1"
              },
              {
                "type": "fakeip",
                "tag": "fakeip",
                "inet4_range": "198.18.0.0/15",
                "inet6_range": "fc00::/18"
              }
            ],
            "rules": [
              {
                "query_type": [
                  "A",
                  "AAAA"
                ],
                "server": "fakeip"
              }
            ]
          }
        }
        ```

=== "RCode"

    === ":material-card-remove: Deprecated"
        
        ```json
        {
          "dns": {
            "servers": [
              {
                "address": "rcode://refused"
              }
            ]
          }
        }
        ```
    
    === ":material-card-multiple: New"
        
        ```json
        {
          "dns": {
            "rules": [
              {
                "domain": [
                  "example.com"
                ],
                // other rules
                
                "action": "predefined",
                "rcode": "REFUSED"
              }
            ]
          }
        }
        ```

=== "Servers with domain address"

    === ":material-card-remove: Deprecated"
        
        ```json
        {
          "dns": {
            "servers": [
              {
                "address": "https://dns.google/dns-query",
                "address_resolver": "google"
              },
              {
                "tag": "google",
                "address": "1.1.1.1"
              }
            ]
          }
        }
        ```
    
    === ":material-card-multiple: New"
    
        ```json
        {
          "dns": {
            "servers": [
              {
                "type": "https",
                "server": "dns.google",
                "domain_resolver": "google"
              },
              {
                "type": "udp",
                "tag": "google",
                "server": "1.1.1.1"
              }
            ]
          }
        }
        ```

=== "Servers with strategy"

    === ":material-card-remove: Deprecated"
            
        ```json
        {
          "dns": {
            "servers": [
              {
                "address": "1.1.1.1",
                "strategy": "ipv4_only"
              },
              {
                "tag": "google",
                "address": "8.8.8.8",
                "strategy": "prefer_ipv6"
              }
            ],
            "rules": [
              {
                "domain": "google.com",
                "server": "google"
              }
            ]
          }
        }
        ```
    
    === ":material-card-multiple: New"
    
        ```json
        {
          "dns": {
            "servers": [
              {
                "type": "udp",
                "server": "1.1.1.1"
              },
              {
                "type": "udp",
                "tag": "google",
                "server": "8.8.8.8"
              }
            ],
            "rules": [
              {
                "domain": "google.com",
                "server": "google",
                "strategy": "prefer_ipv6"
              }
            ],
            "strategy": "ipv4_only"
          }
        }
        ```

=== "Servers with client subnet"

    === ":material-card-remove: Deprecated"
        
        ```json
        {
          "dns": {
            "servers": [
              {
                "address": "1.1.1.1"
              },
              {
                "tag": "google",
                "address": "8.8.8.8",
                "client_subnet": "1.1.1.1"
              }
            ]
          }
        }
        ```
    
    === ":material-card-multiple: New"
    
        ```json
        {
          "dns": {
            "servers": [
              {
                "type": "udp",
                "server": "1.1.1.1"
              },
              {
                "type": "udp",
                "tag": "google",
                "server": "8.8.8.8"
              }
            ],
            "rules": [
              {
                "domain": "google.com",
                "server": "google",
                "client_subnet": "1.1.1.1"
              }
            ]
          }
        }
        ```

### Migrate outbound DNS rule items to domain resolver

The legacy outbound DNS rules are deprecated and can be replaced by new domain resolver options.

!!! info "References"

    [DNS rule](/configuration/dns/rule/#outbound) /
    [Dial Fields](/configuration/shared/dial/#domain_resolver) /
    [Route](/configuration/route/#domain_resolver)

=== ":material-card-remove: Deprecated"

    ```json
    {
      "dns": {
        "servers": [
          {
            "address": "local",
            "tag": "local"
          }
        ],
        "rules": [
          {
            "outbound": "any",
            "server": "local"
          }
        ]
      },
      "outbounds": [
        {
          "type": "socks",
          "server": "example.org",
          "server_port": 2080
        }
      ]
    }
    ```

=== ":material-card-multiple: New"

    ```json
    {
      "dns": {
        "servers": [
          {
            "type": "local",
            "tag": "local"
          }
        ]
      },
      "outbounds": [
        {
          "type": "socks",
          "server": "example.org",
          "server_port": 2080,
          "domain_resolver": {
            "server": "local",
            "rewrite_ttl": 60,
            "client_subnet": "1.1.1.1"
          },
          // or "domain_resolver": "local",
        }
      ],
      
      // or
    
      "route": {
        "default_domain_resolver": {
          "server": "local",
          "rewrite_ttl": 60,
          "client_subnet": "1.1.1.1"
        }
      }
    }
    ```

### Migrate outbound domain strategy option to domain resolver

!!! info "References"

    [Dial Fields](/configuration/shared/dial/#domain_strategy)

The `domain_strategy` option in Dial Fields has been deprecated and can be replaced with the new domain resolver option.

Note that due to the use of Dial Fields by some of the new DNS servers introduced in sing-box 1.12,
some people mistakenly believe that `domain_strategy` is the same feature as in the legacy DNS servers.

=== ":material-card-remove: Deprecated"

    ```json
    {
      "outbounds": [
        {
          "type": "socks",
          "server": "example.org",
          "server_port": 2080,
          "domain_strategy": "prefer_ipv4",
        }
      ]
    }
    ```

=== ":material-card-multiple: New"

    ```json
     {
      "dns": {
        "servers": [
          {
            "type": "local",
            "tag": "local"
          }
        ]
      },
      "outbounds": [
        {
          "type": "socks",
          "server": "example.org",
          "server_port": 2080,
          "domain_resolver": {
            "server": "local",
            "strategy": "prefer_ipv4"
          }
        }
      ]
    }
    ```

## 1.11.0

### Migrate legacy special outbounds to rule actions

Legacy special outbounds are deprecated and can be replaced by rule actions.

!!! info "References"

    [Rule Action](/configuration/route/rule_action/) / 
    [Block](/configuration/outbound/block/) / 
    [DNS](/configuration/outbound/dns)

=== "Block"

    === ":material-card-remove: Deprecated"
    
        ```json
        {
          "outbounds": [
            {
              "type": "block",
              "tag": "block"
            }
          ],
          "route": {
            "rules": [
              {
                ...,
                
                "outbound": "block"
              }
            ]
          }
        }
        ```

    === ":material-card-multiple: New"
    
        ```json
        {
          "route": {
            "rules": [
              {
                ...,
                
                "action": "reject"
              }
            ]
          }
        }
        ```

=== "DNS"

    === ":material-card-remove: Deprecated"
    
        ```json
        {
          "inbound": [
            {
              ...,
              
              "sniff": true
            }
          ],
          "outbounds": [
            {
              "tag": "dns",
              "type": "dns"
            }
          ],
          "route": {
            "rules": [
              {
                "protocol": "dns",
                "outbound": "dns"
              }
            ]
          }
        }
        ```
    
    === ":material-card-multiple: New"
    
        ```json
        {
          "route": {
            "rules": [
              {
                "action": "sniff"
              },
              {
                "protocol": "dns",
                "action": "hijack-dns"
              }
            ]
          }
        }
        ```

### Migrate legacy inbound fields to rule actions

Inbound fields are deprecated and can be replaced by rule actions.

!!! info "References"

    [Listen Fields](/configuration/shared/listen/) /
    [Rule](/configuration/route/rule/) / 
    [Rule Action](/configuration/route/rule_action/) / 
    [DNS Rule](/configuration/dns/rule/) / 
    [DNS Rule Action](/configuration/dns/rule_action/)

=== ":material-card-remove: Deprecated"

    ```json
    {
      "inbounds": [
        {
          "type": "mixed",
          "sniff": true,
          "sniff_timeout": "1s",
          "domain_strategy": "prefer_ipv4"
        }
      ]
    }
    ```

=== ":material-card-multiple: New"

    ```json
    {
      "inbounds": [
        {
          "type": "mixed",
          "tag": "in"
        }
      ],
      "route": {
        "rules": [
          {
            "inbound": "in",
            "action": "resolve",
            "strategy": "prefer_ipv4"
          },
          {
            "inbound": "in",
            "action": "sniff",
            "timeout": "1s"
          }
        ]
      }
    }
    ```

### Migrate destination override fields to route options

Destination override fields in direct outbound are deprecated and can be replaced by route options.

!!! info "References"

    [Rule Action](/configuration/route/rule_action/) /
    [Direct](/configuration/outbound/direct/)

=== ":material-card-remove: Deprecated"

    ```json
    {
      "outbounds": [
        {
          "type": "direct",
          "override_address": "1.1.1.1",
          "override_port": 443
        }
      ]
    }
    ```

=== ":material-card-multiple: New"

    ```json
    {
      "route": {
        "rules": [
          {
            "action": "route-options", // or route
            "override_address": "1.1.1.1",
            "override_port": 443
          }
        ]
      }
    ```

### Migrate WireGuard outbound to endpoint

WireGuard outbound is deprecated and can be replaced by endpoint.

!!! info "References"

    [Endpoint](/configuration/endpoint/) /
    [WireGuard Endpoint](/configuration/endpoint/wireguard/) /
    [WireGuard Outbound](/configuration/outbound/wireguard/)

=== ":material-card-remove: Deprecated"

    ```json
    {
      "outbounds": [
        {
          "type": "wireguard",
          "tag": "wg-out",

          "server": "127.0.0.1",
          "server_port": 10001,
          "system_interface": true,
          "gso": true,
          "interface_name": "wg0",
          "local_address": [
            "10.0.0.1/32"
          ],
          "private_key": "<private_key>",
          "peer_public_key": "<peer_public_key>",
          "pre_shared_key": "<pre_shared_key>",
          "reserved": [0, 0, 0],
          "mtu": 1408
        }
      ]
    }
    ```

=== ":material-card-multiple: New"

    ```json
    {
      "endpoints": [
        {
          "type": "wireguard",
          "tag": "wg-ep",
          "system": true,
          "name": "wg0",
          "mtu": 1408,
          "address": [
            "10.0.0.2/32"
          ],
          "private_key": "<private_key>",
          "listen_port": 10000,
          "peers": [
            {
              "address": "127.0.0.1",
              "port": 10001,
              "public_key": "<peer_public_key>",
              "pre_shared_key": "<pre_shared_key>",
              "allowed_ips": [
                "0.0.0.0/0"
              ],
              "persistent_keepalive_interval": 30,
              "reserved": [0, 0, 0]
            }
          ]
        }
      ]
    }
    ```

## 1.10.0

### TUN address fields are merged

`inet4_address` and `inet6_address` are merged into `address`,
`inet4_route_address` and `inet6_route_address` are merged into `route_address`,
`inet4_route_exclude_address` and `inet6_route_exclude_address` are merged into `route_exclude_address`.

!!! info "References"

    [TUN](/configuration/inbound/tun/)

=== ":material-card-remove: Deprecated"

    ```json
    {
      "inbounds": [
        {
          "type": "tun",
          "inet4_address": "172.19.0.1/30",
          "inet6_address": "fdfe:dcba:9876::1/126",
          "inet4_route_address": [
            "0.0.0.0/1",
            "128.0.0.0/1"
          ],
          "inet6_route_address": [
            "::/1",
            "8000::/1"
          ],
          "inet4_route_exclude_address": [
            "192.168.0.0/16"
          ],
          "inet6_route_exclude_address": [
            "fc00::/7"
          ]
        }
      ]
    }
    ```

=== ":material-card-multiple: New"

    ```json
    {
      "inbounds": [
        {
          "type": "tun",
          "address": [
            "172.19.0.1/30",
            "fdfe:dcba:9876::1/126"
          ],
          "route_address": [
            "0.0.0.0/1",
            "128.0.0.0/1",
            "::/1",
            "8000::/1"
          ],
          "route_exclude_address": [
            "192.168.0.0/16",
            "fc00::/7"
          ]
        }
      ]
    }
    ```

## 1.9.5

### Bundle Identifier updates in Apple platform clients

Due to problems with our old Apple developer account,
we can only change Bundle Identifiers to re-list sing-box apps,
which means the data will not be automatically inherited.

For iOS, you need to back up your old data yourself (if you still have access to it);  
for tvOS, you need to re-import profiles from your iPhone or iPad or create it manually;  
for macOS, you can migrate the data folder using the following command:

```bash
cd ~/Library/Group\ Containers && \ 
  mv group.io.nekohasekai.sfa group.io.nekohasekai.sfavt
```

## 1.9.0

### `domain_suffix` behavior update

For historical reasons, sing-box's `domain_suffix` rule matches literal prefixes instead of the same as other projects.

sing-box 1.9.0 modifies the behavior of `domain_suffix`: If the rule value is prefixed with `.`,
the behavior is unchanged, otherwise it matches `(domain|.+\.domain)` instead.

### `process_path` format update on Windows

The `process_path` rule of sing-box is inherited from Clash,
the original code uses the local system's path format (e.g. `\Device\HarddiskVolume1\folder\program.exe`),
but when the device has multiple disks, the HarddiskVolume serial number is not stable.

sing-box 1.9.0 make QueryFullProcessImageNameW output a Win32 path (such as `C:\folder\program.exe`),
which will disrupt the existing `process_path` use cases in Windows.

## 1.8.0

### :material-close-box: Migrate cache file from Clash API to independent options

!!! info "References"

    [Clash API](/configuration/experimental/clash-api/) / 
    [Cache File](/configuration/experimental/cache-file/)

=== ":material-card-remove: Deprecated"

    ```json
    {
      "experimental": {
        "clash_api": {
          "cache_file": "cache.db", // default value
          "cahce_id": "my_profile2",
          "store_mode": true,
          "store_selected": true,
          "store_fakeip": true
        }
      }
    }
    ```

=== ":material-card-multiple: New"

    ```json
    {
      "experimental"  : {
        "cache_file": {
          "enabled": true,
          "path": "cache.db", // default value
          "cache_id": "my_profile2",
          "store_fakeip": true
        }
      }
    }
    ```

### :material-checkbox-intermediate: Migrate GeoIP to rule-sets

!!! info "References"

    [GeoIP](/configuration/route/geoip/) / 
    [Route](/configuration/route/) / 
    [Route Rule](/configuration/route/rule/) / 
    [DNS Rule](/configuration/dns/rule/) / 
    [rule-set](/configuration/rule-set/)

!!! tip

    `sing-box geoip` commands can help you convert custom GeoIP into rule-sets.

=== ":material-card-remove: Deprecated"

    ```json
    {
      "route": {
        "rules": [
          {
            "geoip": "private",
            "outbound": "direct"
          },
          {
            "geoip": "cn",
            "outbound": "direct"
          },
          {
            "source_geoip": "cn",
            "outbound": "block"
          }
        ],
        "geoip": {
          "download_detour": "proxy"
        }
      }
    }
    ```

=== ":material-card-multiple: New"

    ```json
    {
      "route": {
        "rules": [
          {
            "ip_is_private": true,
            "outbound": "direct"
          },
          {
            "rule_set": "geoip-cn",
            "outbound": "direct"
          },
          {
            "rule_set": "geoip-us",
            "rule_set_ipcidr_match_source": true,
            "outbound": "block"
          }
        ],
        "rule_set": [
          {
            "tag": "geoip-cn",
            "type": "remote",
            "format": "binary",
            "url": "https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-cn.srs",
            "download_detour": "proxy"
          },
          {
            "tag": "geoip-us",
            "type": "remote",
            "format": "binary",
            "url": "https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-us.srs",
            "download_detour": "proxy"
          }
        ]
      },
      "experimental": {
        "cache_file": {
          "enabled": true // required to save rule-set cache
        }
      }
    }
    ```

### :material-checkbox-intermediate: Migrate Geosite to rule-sets

!!! info "References"

    [Geosite](/configuration/route/geosite/) / 
    [Route](/configuration/route/) / 
    [Route Rule](/configuration/route/rule/) / 
    [DNS Rule](/configuration/dns/rule/) / 
    [rule-set](/configuration/rule-set/)

!!! tip

    `sing-box geosite` commands can help you convert custom Geosite into rule-sets.

=== ":material-card-remove: Deprecated"

    ```json
    {
      "route": {
        "rules": [
          {
            "geosite": "cn",
            "outbound": "direct"
          }
        ],
        "geosite": {
          "download_detour": "proxy"
        }
      }
    }
    ```

=== ":material-card-multiple: New"

    ```json
    {
      "route": {
        "rules": [
          {
            "rule_set": "geosite-cn",
            "outbound": "direct"
          }
        ],
        "rule_set": [
          {
            "tag": "geosite-cn",
            "type": "remote",
            "format": "binary",
            "url": "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs",
            "download_detour": "proxy"
          }
        ]
      },
      "experimental": {
        "cache_file": {
          "enabled": true // required to save rule-set cache
        }
      }
    }
    ```
