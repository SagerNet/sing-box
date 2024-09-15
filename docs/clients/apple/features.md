# :material-decagram: Features

#### UI options

* Always On
* Include All Networks (Proxy traffic for LAN and cellular services)
* (Apple tvOS) Import profile from iPhone/iPad

#### Service

SFI/SFM/SFT allows you to run sing-box through NetworkExtension with Application Extension or System Extension.

#### TUN

SFI/SFM/SFT provides an unprivileged TUN implementation through NetworkExtension.

| TUN inbound option            | Available         | Note              |
|-------------------------------|-------------------|-------------------|
| `interface_name`              | :material-close:️ | Managed by Darwin |
| `inet4_address`               | :material-check:  | /                 |
| `inet6_address`               | :material-check:  | /                 |
| `mtu`                         | :material-check:  | /                 |
| `gso`                         | :material-close:  | Not implemented   |
| `auto_route`                  | :material-check:  | /                 |
| `strict_route`                | :material-close:️ | Not implemented   |
| `inet4_route_address`         | :material-check:  | /                 |
| `inet6_route_address`         | :material-check:  | /                 |
| `inet4_route_exclude_address` | :material-check:  | /                 |
| `inet6_route_exclude_address` | :material-check:  | /                 |
| `endpoint_independent_nat`    | :material-check:  | /                 |
| `stack`                       | :material-check:  | /                 |
| `include_interface`           | :material-close:️ | Not implemented   |
| `exclude_interface`           | :material-close:️ | Not implemented   |
| `include_uid`                 | :material-close:️ | Not implemented   |
| `exclude_uid`                 | :material-close:️ | Not implemented   |
| `include_android_user`        | :material-close:️ | Not implemented   |
| `include_package`             | :material-close:️ | Not implemented   |
| `exclude_package`             | :material-close:️ | Not implemented   |
| `platform`                    | :material-check:  | /                 |

| Route/DNS rule option | Available        | Note                  |
|-----------------------|------------------|-----------------------|
| `process_name`        | :material-close: | No permission         |
| `process_path`        | :material-close: | No permission         |
| `process_path_regex`  | :material-close: | No permission         |
| `package_name`        | :material-close: | /                     |
| `user`                | :material-close: | No permission         |
| `user_id`             | :material-close: | No permission         |
| `wifi_ssid`           | :material-alert: | Only supported on iOS |
| `wifi_bssid`          | :material-alert: | Only supported on iOS |

### Chore

* Crash logs is located in `Settings` -> `View Service Log`
