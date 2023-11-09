# :material-decagram: Features

#### UI options

* Always On
* Include All Networks (Proxy traffic for LAN and cellular services)
* (Apple tvOS) Import profile from iPhone/iPad

#### Service

SFI/SFM/SFT allows you to run sing-box through NetworkExtension with Application Extension or System Extension.

#### TUN

SFI/SFM/SFT provides an unprivileged TUN implementation through NetworkExtension.

| TUN inbound option            | Available | Note              |
|-------------------------------|-----------|-------------------|
| `interface_name`              | ✖️        | Managed by Darwin |
| `inet4_address`               | ✔️        | /                 |
| `inet6_address`               | ✔️        | /                 |
| `mtu`                         | ✔️        | /                 |
| `auto_route`                  | ✔️        | /                 |
| `strict_route`                | ✖️        | Not implemented   |
| `inet4_route_address`         | ✔️        | /                 |
| `inet6_route_address`         | ✔️        | /                 |
| `inet4_route_exclude_address` | ✔️        | /                 |
| `inet6_route_exclude_address` | ✔️        | /                 |
| `endpoint_independent_nat`    | ✔️        | /                 |
| `stack`                       | ✔️        | /                 |
| `include_interface`           | ✖️        | Not implemented   |
| `exclude_interface`           | ✖️        | Not implemented   |
| `include_uid`                 | ✖️        | Not implemented   |
| `exclude_uid`                 | ✖️        | Not implemented   |
| `include_android_user`        | ✖️        | Not implemented   |
| `include_package`             | ✖️        | Not implemented   |
| `exclude_package`             | ✖️        | Not implemented   |
| `platform`                    | ✔️        | /                 |

### Chore

* Crash logs is located in `Settings` -> `View Service Log`
