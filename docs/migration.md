---
icon: material/arrange-bring-forward
---

# Migration

## 1.8.0

!!! warning "Unstable"

    This version is still under development, and the following migration guide may be changed in the future.

### :material-close-box: Migrate cache file from Clash API to independent options

!!! info "Reference"

    [Clash API](/configuration/experimental/clash-api) / 
    [Cache File](/configuration/experimental/cache-file)

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

### :material-checkbox-intermediate: Migrate GeoIP to rule sets

!!! info "Reference"

    [GeoIP](/configuration/route/geoip) / 
    [Route](/configuration/route) / 
    [Route Rule](/configuration/route/rule) / 
    [DNS Rule](/configuration/dns/rule) / 
    [Rule Set](/configuration/rule-set)

!!! tip

    `sing-box geoip` commands can help you convert custom GeoIP into rule sets.

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
          "enabled": true // required to save Rule Set cache
        }
      }
    }
    ```

### :material-checkbox-intermediate: Migrate Geosite to rule sets

!!! info "Reference"

    [Geosite](/configuration/route/geosite) / 
    [Route](/configuration/route) / 
    [Route Rule](/configuration/route/rule) / 
    [DNS Rule](/configuration/dns/rule) / 
    [Rule Set](/configuration/rule-set)

!!! tip

    `sing-box geosite` commands can help you convert custom Geosite into rule sets.

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
          "enabled": true // required to save Rule Set cache
        }
      }
    }
    ```