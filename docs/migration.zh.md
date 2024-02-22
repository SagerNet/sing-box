---
icon: material/arrange-bring-forward
---

## 1.9.0

!!! warning "不稳定的"

    该版本仍在开发中，迁移指南可能将在未来更改。

### `domain_suffix` 行为更新

由于历史原因，sing-box 的 `domain_suffix` 规则匹配字面前缀，而不与其他项目相同。

sing-box 1.9.0 修改了 `domain_suffix` 的行为：如果规则值以 `.` 为前缀则行为不变，否则改为匹配 `(domain|.+\.domain)`。

### 对 Windows 上 `process_path` 格式的更新

sing-box 的 `process_path` 规则继承自Clash，
原始代码使用本地系统的路径格式（例如 `\Device\HarddiskVolume1\folder\program.exe`），
但是当设备有多个硬盘时，该 HarddiskVolume 系列号并不稳定。

sing-box 1.9.0 使 QueryFullProcessImageNameW 输出 Win32 路径（如 `C:\folder\program.exe`），
这将会破坏现有的 Windows `process_path` 用例。

## 1.8.0

### :material-close-box: 将缓存文件从 Clash API 迁移到独立选项

!!! info "参考"

    [Clash API](/zh/configuration/experimental/clash-api/) / 
    [Cache File](/zh/configuration/experimental/cache-file/)

=== ":material-card-remove: 弃用的"

    ```json
    {
      "experimental": {
        "clash_api": {
          "cache_file": "cache.db", // 默认值
          "cahce_id": "my_profile2",
          "store_mode": true,
          "store_selected": true,
          "store_fakeip": true
        }
      }
    }
    ```

=== ":material-card-multiple: 新的"

    ```json
    {
      "experimental"  : {
        "cache_file": {
          "enabled": true,
          "path": "cache.db", // 默认值
          "cache_id": "my_profile2",
          "store_fakeip": true
        }
      }
    }
    ```

### :material-checkbox-intermediate: 迁移 GeoIP 到规则集

!!! info "参考"

    [GeoIP](/zh/configuration/route/geoip/) / 
    [路由](/zh/configuration/route/) / 
    [路由规则](/zh/configuration/route/rule/) / 
    [DNS 规则](/zh/configuration/dns/rule/) / 
    [规则集](/zh/configuration/rule-set/)

!!! tip

    `sing-box geoip` 命令可以帮助您将自定义 GeoIP 转换为规则集。

=== ":material-card-remove: 弃用的"

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

=== ":material-card-multiple: 新的"

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

### :material-checkbox-intermediate: 迁移 Geosite 到规则集

!!! info "参考"

    [Geosite](/zh/configuration/route/geosite/) / 
    [路由](/zh/configuration/route/) / 
    [路由规则](/zh/configuration/route/rule/) / 
    [DNS 规则](/zh/configuration/dns/rule/) / 
    [规则集](/zh/configuration/rule-set/)

!!! tip

    `sing-box geosite` 命令可以帮助您将自定义 Geosite 转换为规则集。

=== ":material-card-remove: 弃用的"

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

=== ":material-card-multiple: 新的"

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